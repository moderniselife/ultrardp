# Fix for Upside-Down Streams

## Task Description
Identify and fix the issue causing streams to be rendered upside down in the simple client implementation (`cmd/simpleclient/cmd_client.go`).

## Problem Analysis
The streams were being rendered upside down because of a coordinate system mismatch between OpenGL and image data:

1. OpenGL uses a coordinate system where (0,0) is at the bottom-left corner of the screen.
2. Image data (JPEG/PNG) uses a coordinate system where (0,0) is at the top-left corner.

This mismatch causes the image to appear "flipped" vertically when rendering in OpenGL without coordinate adjustment.

## Solution Implemented
The solution was to flip the Y-coordinates in the texture mapping when rendering the image:

1. Modified the texture coordinate mapping in the `renderSimpleFullscreenTexture` function:
   - Changed from mapping (0,0) texture coordinates to (0,0) vertex coordinates
   - To mapping (0,1) texture coordinates to (0,0) vertex coordinates (flipping the Y-axis)

2. Updated comments to explain the coordinate system differences and why the flip is necessary.

## Code Changes

In `cmd/simpleclient/cmd_client.go`, the texture coordinate mapping in the `renderSimpleFullscreenTexture` function was modified:

```go
// Before:
// Standard texture coordinates - [0,0] at bottom-left
gl.TexCoord2f(0.0, 0.0); gl.Vertex2f(0.0, 0.0) // Bottom-left
gl.TexCoord2f(1.0, 0.0); gl.Vertex2f(1.0, 0.0) // Bottom-right
gl.TexCoord2f(1.0, 1.0); gl.Vertex2f(1.0, 1.0) // Top-right
gl.TexCoord2f(0.0, 1.0); gl.Vertex2f(0.0, 1.0) // Top-left

// After:
// OpenGL has (0,0) at bottom-left, but image data (JPEG/PNG) has (0,0) at top-left
// Flip Y-coordinates to fix the upside-down rendering
gl.TexCoord2f(0.0, 1.0); gl.Vertex2f(0.0, 0.0) // Bottom-left of screen, top-left of texture
gl.TexCoord2f(1.0, 1.0); gl.Vertex2f(1.0, 0.0) // Bottom-right of screen, top-right of texture
gl.TexCoord2f(1.0, 0.0); gl.Vertex2f(1.0, 1.0) // Top-right of screen, bottom-right of texture
gl.TexCoord2f(0.0, 0.0); gl.Vertex2f(0.0, 1.0) // Top-left of screen, bottom-left of texture
```

## Verification
After making these changes, the streams should now be rendered right-side up, with the correct orientation matching the original image data.

## Additional Notes
This is a common issue when working with OpenGL and image data from other sources. The key insight is understanding the different coordinate systems:

- OpenGL: Origin (0,0) at bottom-left, +Y goes up
- Image formats: Origin (0,0) at top-left, +Y goes down

When mapping between these systems, the Y-coordinate typically needs to be flipped to maintain the correct orientation.