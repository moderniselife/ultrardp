# Fix Multiple Client Issues: JPEG Decoding, Window Creation, and Rendering

## Initial Task Description
Fix the client-side issues in the UltraRDP application:
1. Windows remaining black with console logs showing: "invalid JPEG format: missing SOI marker" errors
2. After fixing the JPEG error, windows not opening at all or inconsistently opening
3. Window positioning issues where all windows appeared at the same coordinates

## Root Cause Analysis

### Issue 1: JPEG Decoding Error
The initial issue was identified as a double-decode problem in the frame handling logic:

1. The client receives JPEG-encoded frames from the server.
2. In `client.go`, the `updateFrameBuffer` method decoded the JPEG data into RGBA pixels, and stored these raw pixels in the frame buffer.
3. Later in `display.go`, the `renderFrame` method attempted to decode the frame buffer data as JPEG again, but the data was no longer in JPEG format (it was already raw RGBA pixels), so it failed with "missing SOI marker" error.

The SOI (Start of Image) marker (0xFF, 0xD8) is required at the beginning of every valid JPEG file but was missing because the data had already been decoded to raw pixel format.

### Issue 2: Window Creation Problems
After fixing the JPEG decoding issue, a new problem appeared - the windows did not open consistently. The application connected to the server and received frame data as shown in the logs, but windows did not display reliably. Analysis revealed:

1. Incompatible OpenGL version (4.1) that wasn't supported on all hardware
2. Full-screen mode causing issues on some configurations
3. Window dimensions too large for GLFW to handle properly
4. Insufficient event processing between window creations
5. Inadequate error handling in the window creation process
6. Missing validation for monitor mismatches
7. Inconsistent window positioning (all at the same coordinates)

### Issue 3: Rendering Problems
Even after fixing the window creation issues, windows would sometimes appear black despite the frame data being properly received. This pointed to issues in the rendering pipeline, including potential OpenGL errors, texture binding issues, or JPEG decoding problems not being properly reported.

Further investigation showed:
- The initial fix worked for the first run, but often failed on subsequent launches
- Windows were getting created but not properly initialized or were being created with dimensions too large for the system to handle
- GLFW wasn't getting time to process events between window creations
- OpenGL errors weren't being detected and reported properly

## Solution Implementation

### Phase 1: Fix for JPEG Decoding
Modified the JPEG handling flow to:

1. Store the raw JPEG data in the frame buffer without decoding it in `updateFrameBuffer`
2. Only decode the JPEG data once in `renderFrame` when it's needed for display
3. Add validation for the JPEG SOI marker in both methods to catch potential issues early

### Phase 2: Basic Improvements to Window Creation
Added improved window creation and OpenGL handling:

1. Changed OpenGL version from 4.1 to 3.3 for better compatibility
2. Added handling for GLFW monitor count mismatches
3. Added null pointer checks and better error messages
4. Fixed window positioning issues
5. Added code to skip rendering when no frame data is available
6. Updated shader versions to match the OpenGL version (330 instead of 410)
7. Improved error handling in the rendering loop

### Phase 3: Complete Window Creation Overhaul
Since the previous changes didn't fully resolve the window creation issues, a more comprehensive solution was implemented:

1. Implemented a multi-stage window creation process with fallbacks:
   - Try OpenGL 3.3 first (preferred version)
   - Fall back to OpenGL 2.1 if 3.3 fails (for older GPUs)
   - Use compatibility profile as a last resort (for maximum compatibility)

2. Improved window dimensions and properties:
   - Limited window dimensions to 1280x720 (previously fullscreen)
   - Used windowed mode instead of fullscreen for more reliable startup
   - Added window borders (decorated) for easier troubleshooting and handling

3. Added process management for GLFW:
   - Added explicit delays between window creation to let the system process events
   - Added event polling after each window creation
   - Added tracking of successfully initialized windows for better diagnostics

### Phase 4: Rendering and Positioning Fixes
Added extensive rendering improvements to ensure frames display correctly:

1. Added OpenGL error checking throughout the rendering process
2. Implemented texture and VAO validation before drawing
3. Added a window position correction function that runs during the main loop
4. Enhanced logging of JPEG decoding process with detailed diagnostics
5. Added explicit handling of rendering errors

## Code Changes

### 1. Changes to client/client.go

Modified `updateFrameBuffer` to:
- Validate JPEG data by checking for the SOI marker (0xFF, 0xD8)
- Store the raw JPEG data instead of decoding it
- Add better error logging

```diff
--- client/client.go (before)
+++ client/client.go (after)
@@ -202,27 +202,19 @@ func (c *Client) updateFrameBuffer(serverMonitorID uint32, frameData []byte) {
     c.frameMutex.Lock()
     defer c.frameMutex.Unlock()
     
-    // Map server monitor ID to local monitor ID
+    // Map server monitor ID to local monitor ID
     localMonitorID, ok := c.monitorMap[serverMonitorID]
     if !ok {
         log.Printf("No mapping found for server monitor ID %d", serverMonitorID)
         return
     }
     
-    // Decode JPEG frame data
-    img, err := jpeg.Decode(bytes.NewReader(frameData))
-    if err != nil {
-        log.Printf("Error decoding JPEG frame: %v", err)
-        return
-    }
-
-    // Convert to RGBA
-    bounds := img.Bounds()
-    rgba := image.NewRGBA(bounds)
-    draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
-
-    // Store the RGBA pixel data
-    c.frameBuffers[localMonitorID] = rgba.Pix
+    // Validate JPEG header (SOI marker: FF D8)
+    if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
+        log.Printf("Invalid JPEG data received: missing SOI marker")
+        return
+    }
     
-    log.Printf("Updated frame buffer for monitor %d with %d bytes of RGBA data", localMonitorID, len(rgba.Pix))
+    // Store the raw JPEG data for rendering later
+    c.frameBuffers[localMonitorID] = frameData
+    log.Printf("Updated frame buffer for monitor %d with %d bytes of JPEG data", localMonitorID, len(frameData))
 }
```

### 2. Changes to client/display.go

#### A. Updated OpenGL Import
```diff
--- client/display.go
+++ client/display.go
@@ -9,7 +9,7 @@ import (
 	"log"
 	"runtime"
 
-	"github.com/go-gl/gl/v4.1-core/gl"
+	"github.com/go-gl/gl/v3.3-core/gl"
 	"github.com/go-gl/glfw/v3.3/glfw"
 )
```

#### B. Multi-stage Window Creation with Fallbacks
```go
// Try three different OpenGL versions in order of preference
var window *glfw.Window = nil
var err error

// Try OpenGL 3.3 first (preferred)
log.Printf("Attempting window creation with OpenGL 3.3")
glfw.DefaultWindowHints()
glfw.WindowHint(glfw.Resizable, glfw.False)
glfw.WindowHint(glfw.Decorated, glfw.True) // Use decorated for better compatibility
glfw.WindowHint(glfw.ContextVersionMajor, 3)
glfw.WindowHint(glfw.ContextVersionMinor, 3)
glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

// Try creating window for this monitor
window, err = glfw.CreateWindow(
    windowWidth,
    windowHeight,
    fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID), 
    glfwMonitor, // Use the monitor we identified (can be nil)
    nil,
)

// If OpenGL 3.3 failed, try OpenGL 2.1 (backup)
if err != nil {
    log.Printf("OpenGL 3.3 window creation failed: %v", err)
    log.Printf("Attempting fallback to OpenGL 2.1...")
    
    glfw.DefaultWindowHints()
    glfw.WindowHint(glfw.Resizable, glfw.False)
    glfw.WindowHint(glfw.Decorated, glfw.True)
    glfw.WindowHint(glfw.ContextVersionMajor, 2)
    glfw.WindowHint(glfw.ContextVersionMinor, 1)
    
    // Try again with OpenGL 2.1
    window, err = glfw.CreateWindow(
        windowWidth,
        windowHeight,
        fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
        glfwMonitor,
        nil,
    )
}

// Add delay between window creation operations to give GLFW time to process
time.Sleep(100 * time.Millisecond)
```

#### C. Window Positioning and Size Limiting
```go
// Calculate window dimensions - cap at 1280x720 for better compatibility
windowWidth := int(monitor.Width)
windowHeight := int(monitor.Height)

if windowWidth > 1280 {
    windowWidth = 1280
    log.Printf("Limiting window width to 1280 pixels for better compatibility")
}

if windowHeight > 720 {
    windowHeight = 720
    log.Printf("Limiting window height to 720 pixels for better compatibility")
}

// ...later after creating window:

// Get the actual window dimensions and record for positioning
var width, height int
width, height = window.GetSize()
log.Printf("Actual window dimensions: %dx%d", width, height)
window.SetTitle(fmt.Sprintf("UltraRDP - Monitor %d (%dx%d)", monitor.ID, width, height))

// Force window to be windowed mode (not fullscreen) for better positioning
window.SetAttrib(glfw.Decorated, glfw.True)

window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
log.Printf("Window position set to %d,%d", int(monitor.PositionX), int(monitor.PositionY))
```

#### D. Continuous Window Position Monitoring
```go
// Function to check and update window positions
updateWindowPositions := func() {
    for i, window := range c.windows {
        if window == nil || i >= int(c.localMonitors.MonitorCount) {
            continue
        }
        
        // Get the monitor info for this window
        monitor := c.localMonitors.Monitors[i]
        
        // Get current window position
        x, y := window.GetPos()
        
        // Update position if it doesn't match monitor position
        if x != int(monitor.PositionX) || y != int(monitor.PositionY) {
            log.Printf("Repositioning window %d to %d,%d (was at %d,%d)", 
                i, monitor.PositionX, monitor.PositionY, x, y)
            window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
        }
    }
}
```

#### E. Enhanced Error Checking in Rendering
```go
// Print header bytes to debug the JPEG data
headerStr := ""
for i := 0; i < min(16, len(frameData)); i++ {
    headerStr += fmt.Sprintf("%02X ", frameData[i])
}
log.Printf("JPEG header bytes: %s", headerStr)

// Validate JPEG format (check for SOI marker)
if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
    log.Printf("Error: Invalid JPEG format in renderFrame: missing SOI marker")
    // Clear window if frame data is invalid
    gl.ClearColor(0.0, 0.0, 0.0, 1.0)
    gl.Clear(gl.COLOR_BUFFER_BIT)
    window.SwapBuffers()
    return
}

// Check for OpenGL errors
var glErr uint32
if glErr = gl.GetError(); glErr != gl.NO_ERROR {
    log.Printf("OpenGL error before binding texture: 0x%x", glErr)
}

// Verify texture ID is valid
if texture == 0 {
    log.Printf("Error: Invalid texture ID 0")
    return
}
```

## Testing
The solution was tested in multiple stages:

1. The initial fix for JPEG decoding worked correctly, showing frames without "SOI marker" errors
2. First window creation improvements worked initially but weren't reliable on reconnection
3. The complete window creation overhaul with OpenGL fallbacks improved reliability
4. The final position correction and rendering enhancements fixed the black window issue

The client now properly connects to the server, displays windows for each monitor, and shows the screen content from the server - even across multiple connect/disconnect cycles.