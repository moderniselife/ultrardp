# Fixing Black Display Issues in RDP Client

## Task Description

Windows were either not opening correctly or displaying black screens instead of showing remote desktop content. This task involved diagnosing and fixing the issue to allow the client to properly display frames received from the server.

## Problem Analysis

After implementing diagnostic code and analyzing logs, we uncovered multiple issues:

1. **Server-side Issue**: Monitor 3 had invalid screen coordinates (4294964736,0) that were causing screenshot capture to fail.
2. **OpenGL Initialization Problem**: There was a segmentation fault during window creation due to improper OpenGL initialization.
3. **Frame Transmission Problems**: Frames were being captured but not properly processed and displayed by the client.

## Steps Taken

### Server-Side Fixes

1. Added coordinate validation to detect unreasonable values:
   ```go
   isValidCoords := true
   if monitor.PositionX > 10000 || monitor.PositionY > 10000 {
       log.Printf("WARNING: Invalid monitor coordinates detected for monitor %d: (%d,%d)",
           monitor.ID, monitor.PositionX, monitor.PositionY)
       isValidCoords = false
   }
   ```

2. Implemented multiple capture methods based on the monitor's coordinate validity:
   ```go
   if isValidCoords {
       // Try with coordinates first if they seem valid
       bound := image.Rect(int(monitor.PositionX), int(monitor.PositionY),
           int(monitor.PositionX)+int(monitor.Width), int(monitor.PositionY)+int(monitor.Height))
       
       log.Printf("Capturing monitor %d with bounds: %v", monitor.ID, bound)
       img, err = screenshot.CaptureRect(bound)
   } else {
       // For monitors with suspect coordinates, use display index directly
       if displayIndex >= 0 && displayIndex < screenshot.NumActiveDisplays() {
           log.Printf("Capturing monitor %d using display index %d", monitor.ID, displayIndex)
           img, err = screenshot.CaptureDisplay(displayIndex)
       }
   }
   ```

3. Added detailed frame validation and diagnostic saving:
   - Added black frame detection to identify capture issues
   - Saved debug frames to help diagnose issues
   - Enhanced error handling and retries

### Client-Side Fixes

1. **Fixed Window Creation and OpenGL Initialization Sequence**:
   - Created all windows first before initializing OpenGL
   - Fixed the order of operations to ensure a valid context before calling OpenGL functions
   - Delayed OpenGL initialization until after windows are created:
   ```go
   // Only after all windows are created, initialize OpenGL and create textures
   if windowCount > 0 {
       // Make first window's context current
       c.windows[0].MakeContextCurrent()
       
       // Initialize OpenGL
       if err := gl.Init(); err != nil {
           return fmt.Errorf("failed to initialize OpenGL: %v", err)
       }
       
       version := gl.GoStr(gl.GetString(gl.VERSION))
       fmt.Printf("OpenGL initialized: %s\n", version)
   }
   ```

2. **Improved Frame Buffer Handling**:
   - Fixed memory management for frame buffers
   - Used copy-on-read to prevent race conditions
   ```go
   // Make a copy of the frame data so we can release the mutex quickly
   frameDataCopy := make([]byte, len(frameData))
   copy(frameDataCopy, frameData)
   c.frameMutex.Unlock()
   ```

3. **Enhanced Rendering Pipeline**:
   - Simplified OpenGL state management:
   ```go
   // Reset OpenGL state completely
   gl.GetError() // Clear any previous errors
   
   // Disable everything that could interfere
   gl.Disable(gl.DEPTH_TEST)
   gl.Disable(gl.CULL_FACE)
   gl.Disable(gl.BLEND)
   gl.Disable(gl.LIGHTING)
   
   // Set up a simple orthographic projection
   gl.MatrixMode(gl.PROJECTION)
   gl.LoadIdentity()
   gl.Ortho(0, 1, 0, 1, -1, 1)
   ```
   - Added proper texture management with error checking
   - Implemented fallback rendering for missing frames

4. **Added Comprehensive Diagnostics**:
   - Added frame saving for debugging
   - Implemented FPS tracking
   - Added error logging and recovery at each step

## Results and Verification

### Server-Side Improvements

The server now properly detects invalid monitor coordinates and automatically switches to a display index-based capture method:

```
WARNING: Invalid monitor coordinates detected for monitor 3: (4294964736,0)
Capturing monitor 3 using display index 2
```

### Client-Side Improvements

1. **Solved Segmentation Fault**:
   - Fixed the sequence of window creation and OpenGL initialization
   - Ensure proper context handling before calling OpenGL functions
   - Added appropriate error checking and recovery

2. **Fixed Black Display Issue**:
   - Implemented proper texture upload and rendering
   - Added clear color to identify when frames are available
   - Improved JPEG decoding and error handling

## Lessons Learned

1. **OpenGL Initialization Sequence is Critical**:
   - OpenGL functions cannot be called before a valid context is made current
   - When creating multiple windows, ensure proper context management
   - Use a consistent initialization flow (windows first, then OpenGL)

2. **Coordinate Validation is Essential**:
   - Always check for reasonable coordinate values
   - Implement fallback mechanisms for invalid coordinates
   - Look for potential integer overflow/underflow issues

3. **Diagnostic Tools are Invaluable**:
   - Frame saving at each stage helped identify issues
   - Error logging provided crucial context for debugging
   - Visual verification with blue/black backgrounds helped confirm logic

## Next Steps

1. Implement more robust monitor mapping between server and client
2. Add proper scaling to handle different monitor resolutions
3. Optimize JPEG encoding/decoding for better performance
4. Consider hardware acceleration for screen capture and rendering