# Video-player toolkit integration notes

Version audited: `uitoolkit v0.23.4`, `paintengine2d v0.11.0`.

The current public widgets and painting APIs are sufficient for this player.
Buttons, sliders and the virtualized playlist keep toolkit input, focus and
accessibility behavior. The app supplies artwork through `Button.Painter`,
`Slider.Painter`, `ListView.RowPaint` and embedded original skin packs. Fullscreen,
resize, file dialogs and drops use public window/widget APIs. Original
artwork is placed through DrawSkinLayout, DrawSkinSlot, and SkinSlotRect;
Button.Shaper gives circular bitmap controls their original hit area.
DecorationsNone and SetShapeFunc preserve the skin silhouette, and
StartMove/StartResize provide desktop dragging and resizing.

## Video surfaces

The public API does not expose a dedicated video surface, libmpv render
integration, native child-window handle, or app-managed GPU texture handoff.
The player therefore owns a libmpv software render context and copies decoded
frames into a paint-engine image. This provides embedded video across toolkit
window backends, with a 1280 × 720 output cap to bound CPU and copy cost.

A future public GPU surface that accepts a renderer callback or shared texture
would allow full-resolution playback without the intermediate CPU buffer.
The decoder remains an application responsibility. Useful requirements are
an explicit render-thread lifecycle, resize/DPI notifications, compositing
with ordinary controls, damage notification, and safe cleanup before closing
the native window.

## File picker

The current `FileDialogOptions.OnPick` returns a single path. The player can
append several files through CLI arguments, file-manager drops, or a folder
scan. A future multiple-selection picker could expose `OnPickMany([]string)`
while retaining the existing single-file callback.

## Validation

All appearances are exercised headlessly at 1× and 1.75×. Real libmpv tests
verify decoded video frames and transport without a display or audio device.
Native macOS and Windows playback remain to be validated on those systems.
