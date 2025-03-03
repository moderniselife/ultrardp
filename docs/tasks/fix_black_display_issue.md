# Fix Black Display Issue in UltraRDP Client

## Task Description
Windows were opening successfully but displaying black screens instead of showing the remote desktop content. This issue occurred after a previous fix for JPEG decode errors (documented in `fix_jpeg_decode_error.md`).

## Root Cause Analysis
After reviewing the code and documentation, I identified that while the JPEG handling was fixed to properly store and handle raw JPEG data, the rendering part of the display was still using OpenGL 3.3 Core Profile with shaders. According to the previous fix documentation, the rendering should have been changed to use OpenGL 2.1 with a fixed-function pipeline for better compatibility across systems.

Specifically:
1. The code was still importing and using OpenGL 3.3 Core Profile libraries
2. The GLFW window hints were still set for OpenGL 3.3
3. The rendering code was still using shaders and VAOs instead of the simpler fixed-function pipeline

## Solution Implementation

Made the following changes to `client/display.go`:

1. Changed the OpenGL import from v3.3-core to v2.1:
```diff
- "github.com/go-gl/gl/v3.3-core/gl"
+ "github.com/go-gl/gl/v2.1/gl"
```

2. Modified the GLFW window hints to target OpenGL 2.1:
```diff
- glfw.WindowHint(glfw.ContextVersionMajor, 3)
- glfw.WindowHint(glfw.ContextVersionMinor, 3)
- glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
- glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
+ glfw.WindowHint(glfw.ContextVersionMajor, 2)
+ glfw.WindowHint(glfw.ContextVersionMinor, 1)
```

3. Replaced the shader-based rendering with fixed-function pipeline:
   - Removed all shader compilation and program linking code
   - Added matrix mode setup (projection and modelview)
   - Changed to immediate mode rendering with gl.Begin/gl.End for the textured quad

## Testing
The changes were verified to ensure:
1. Windows now display the remote desktop content instead of black screens
2. OpenGL 2.1 compatibility works across different systems
3. The rendering code properly shows the JPEG frames sent from the server

## Additional Notes
This solution follows the approach outlined in the previous fix documentation, using the simpler and more widely compatible OpenGL 2.1 fixed-function pipeline instead of the modern shader-based approach. This ensures maximum compatibility across different systems and graphics hardware.