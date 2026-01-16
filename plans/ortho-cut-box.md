# Product Requirement: Ortho Cut Box for Model Viewer

## Summary
Add an interactive **Cut Box** tool to the Model Viewer that lets users define an axis-aligned 3D box for trimming/removing model parts. The box is edited using **two orthographic 2D views** (not 3D handles), making min/max coordinates fast and accurate to author.

## Goals
- Provide a **precise, low-friction** workflow to define Cut Box bounds.
- Make editing usable without guessing coordinates or trial-and-error scripting.
- Keep the existing viewer features unchanged (cameras/animations/grid/collisions/fullscreen).

## Non-Goals
- No mesh editing inside the viewer beyond defining the box.
- No advanced selection shapes (sphere, lasso, arbitrary planes).

## User Experience
- **Placement**: A new column appears **to the right of the existing Cameras/Animations panel**.
- **Enable/Disable**: A toolbar toggle enables/disables Cut Box mode.
- **3D Preview**: When enabled, the 3D viewport shows the box as a clear overlay for context.

## Ortho Editors (Core)
Two embedded orthographic views control the box:

### 1) Top View (X/Y)
- Shows the Cut Box projection on the **X/Y plane** (top-down).
- Users can **hover/select** each rectangle side and **drag** to move that side.
- Dragging updates **min/max X** and **min/max Y**.
- Supports **zoom in/out** within the view.

### 2) Side View (X/Z)
- Shows the Cut Box projection on the **X/Z plane** (side).
- Users can **hover/select** each rectangle side and **drag** to move that side.
- Dragging updates **min/max X** and **min/max Z**.
- Supports **zoom in/out** within the view.

## Controls & Feedback
- **Hover highlight**: The side under the cursor highlights clearly.
- **Drag behavior**: Dragging a side moves only that boundary.
- **Bounds safety**: Prevent invalid boxes (e.g., min exceeding max).
- **Reset**: One action resets the box to the model’s current bounds.

## Output
- **Copy code**: A button copies a snippet in the format:
  - `model.modify.deleteVerticesInsideBox([minX, minY, minZ], [maxX, maxY, maxZ]);`
- Values are displayed/copied in a human-friendly numeric format (e.g., rounded).

## Acceptance Criteria
- Users can define a box entirely using the two ortho views (no 3D dragging required).
- Zoom works in each view without scrolling the surrounding container.
- Hover/drag targets align with the visible sides (no offset/mismatch).
- Copy output reflects the current box values and is paste-ready.

