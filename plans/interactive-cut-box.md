# Product Requirement Document: Interactive Cut Box Tool for Model Viewer

## 1. Problem Statement
When editing models using the `delete-cut-crop` script, users need to specify precise 3D coordinates (min/max bounds) to cut or delete parts of the model. Finding these coordinates manually by trial and error is time-consuming and inefficient, especially when multiple cuts (e.g., 20+) are required.

## 2. Goal
Implement an interactive "Cut Box" tool within the existing `Model Viewer` UI. This tool will allow users to visualize an axis-aligned bounding box (AABB) on top of the model, adjust its size and position via draggable handles and numeric inputs, and generate the corresponding code snippet for the `deleteVerticesInsideBox` function.

## 3. User Stories
- **As a user**, I want to toggle a "Cut Box" mode in the viewer so I can see a selection box overlaid on the model.
- **As a user**, I want to drag the faces or corners of the box to resize it visually to match the area I want to remove.
- **As a user**, I want to fine-tune the coordinates via text inputs for precision.
- **As a user**, I want to see the current `min` and `max` coordinates of the box.
- **As a user**, I want to copy the generated TypeScript code snippet to my clipboard to paste into my script.

## 4. Functional Requirements

### 4.1. UI Components
- **Toggle Button**: A button in the viewer toolbar (next to Grid/Collision toggles) to enable/disable the Cut Box tool. Icon: Box with scissors or similar.
- **Sidebar Panel**: When the tool is active, a new section in the sidebar (or a floating panel) should appear containing:
  - **Enable/Disable Toggle** (mirrors toolbar).
  - **Coordinate Inputs**: Two rows of XYZ inputs for `Min` and `Max` bounds.
  - **Copy Code Button**: A button to copy `model.modify.deleteVerticesInsideBox(...)` snippet.
  - **Reset Button**: Resets the box to the model's bounding box.

### 4.2. 3D Visualization
- **Wireframe Box**: Render an axis-aligned box representing the current selection.
  - Color: Distinct (e.g., Orange or Cyan) to contrast with the model and grid.
  - Style: Wireframe or semi-transparent solid.
- **Interactable Handles**:
  - **Face Handles**: 6 handles (one for each face: ±X, ±Y, ±Z) to drag and resize the box along a single axis.
  - **Visual Feedback**: Highlight handles on hover.

### 4.3. Interaction
- **Dragging**: Clicking and dragging a handle should update the corresponding coordinate (Min or Max) in real-time.
- **Raycasting**: Implement raycasting to detect mouse interaction with the handles.
- **Orbit Controls Compatibility**: Ensure dragging handles doesn't conflict with camera rotation/panning. (e.g., disable camera controls while dragging a handle).

### 4.4. Code Generation
- Format: `model.modify.deleteVerticesInsideBox([minX, minY, minZ], [maxX, maxY, maxZ]);`
- Precision: Round to 2 decimal places.

## 5. Technical Implementation Plan

### 5.1. Data Model
- Add state to `ModelViewerUi`:
  ```typescript
  interface CutBoxState {
    visible: boolean;
    min: [number, number, number];
    max: [number, number, number];
  }
  const [cutBox, setCutBox] = useState<CutBoxState>({ ... });
  ```

### 5.2. Rendering (WebGL/MDLX)
- **Box Primitive**: Use `mdlx.createPrimitive` (similar to `createGridModel`) to create the wireframe box and handles.
- **Dynamic Updates**: Since `mdlx` primitives are static, we might need to:
  - Scale/Translate a unit cube instance to match the `min`/`max` bounds.
  - `Location = (min + max) / 2`
  - `Scale = (max - min)`
- **Handles**:
  - Create small cube or sphere primitives for handles.
  - Position them at the center of each face.
  - `Face +X`: `(max.x, center.y, center.z)`
  - `Face -X`: `(min.x, center.y, center.z)`
  - ...and so on.

### 5.3. Input Handling
- Extend `onMouseDown`, `onMouseMove`, `onMouseUp` in `ModelViewerUi`.
- **Hit Test**:
  - Project mouse coordinates to a ray.
  - Check intersection with handle instances (if `mdx-m3-viewer` supports raycasting against instances, otherwise implement simple ray-box intersection for the handles).
- **State Update**:
  - On drag start: Record initial mouse pos and active handle.
  - On drag move: Calculate delta, apply to specific axis of `min` or `max`.
  - On drag end: Commit change.

### 5.4. Integration
- Modify `wow-converter/webui/components/common/model-viewer.tsx`.
- Add the new UI controls to the sidebar.
- Hook into the render loop/scene setup to add the Cut Box instances.

## 6. Milestones
1.  **Phase 1**: UI Controls & State (Sidebar inputs, Toggle).
2.  **Phase 2**: Visualizing the Box (Static rendering based on inputs).
3.  **Phase 3**: Interactive Handles (Dragging handles to update state).
4.  **Phase 4**: Code Generation & Polish.

