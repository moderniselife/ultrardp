# Fix JPEG Decode Error in RDP Client

## Task Description
Fix the issue in the RDP client where windows remain black and client console logs report:
```
UltraRDP: 2025/03/02 20:52:53 Error decoding JPEG frame: invalid JPEG format: missing SOI marker
```

## Root Cause Analysis
After examining the codebase, the issue was identified as a double-decode problem in the frame handling logic:

1. The client receives JPEG-encoded frames from the server.
2. In `client.go`, the `updateFrameBuffer` method decodes the JPEG data into RGBA pixels, then stores these raw pixels in the frame buffer.
3. Later in `display.go`, the `renderFrame` method attempts to decode the frame buffer data as JPEG again, but the data is no longer in JPEG format (it's already raw RGBA pixels), so it fails with "missing SOI marker" error.

The SOI (Start of Image) marker (0xFF, 0xD8) is required at the beginning of every valid JPEG file but was missing because the data had already been decoded to raw pixel format.

## Solution
Modified the frame handling flow to:

1. Store the raw JPEG data in the frame buffer without decoding it in `updateFrameBuffer`.
2. Only decode the JPEG data once in `renderFrame` when it's needed for display.
3. Added validation for the JPEG SOI marker in both methods to catch potential issues early.

This ensures that rendering only attempts to decode valid JPEG data and avoids the double-decode issue.

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

Modified `renderFrame` to:
- Validate JPEG data before attempting to decode
- Updated comments to clarify the decoding process
- Added better error handling

```diff
--- client/display.go (before)
+++ client/display.go (after)
@@ -211,7 +211,19 @@ func (c *Client) renderFrame(window *glfw.Window, frameData []byte, texture, vao
         return
     }

-    // Decode JPEG frame data
+    // Validate JPEG format (check for SOI marker)
+    if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
+        log.Printf("Error: Invalid JPEG format in renderFrame: missing SOI marker")
+        // Clear window if frame data is invalid
+        gl.ClearColor(0.0, 0.0, 0.0, 1.0)
+        gl.Clear(gl.COLOR_BUFFER_BIT)
+        window.SwapBuffers()
+        return
+    }
+
+    // Decode JPEG frame data - note that frameData is now raw JPEG data from the server
+    // We no longer decode in updateFrameBuffer, only in renderFrame
     img, err := jpeg.Decode(bytes.NewReader(frameData))
     if err != nil {
         log.Printf("Error decoding JPEG frame: %v", err)
```

## Testing
To test the fix, run the RDP client and server. The windows should now properly display frames instead of remaining black, and the "missing SOI marker" errors should no longer appear in the client console logs.