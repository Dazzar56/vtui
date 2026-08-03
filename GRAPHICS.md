# Images

`vtui` can paint bitmaps on top of the character grid. The design keeps the
pixels and the transport strictly apart, because the same picture has to reach
a kitty terminal, an iTerm2 window, a sixel console and a native X11 / Wayland
/ gogpu window, and only the last of those wants raw memory.

## Pieces

* `ImageSurface` — a top-down RGBA8 buffer. It knows nothing about file
  formats: decoding belongs to the application, `vtui` only ships bytes.
  A cheap content hash lets backends notice a surface they already sent.
* `ImagePlacement` — one surface (or a crop of it) shown over a rectangle of
  **cells**. Cell geometry is what makes a placement portable: the terminal
  backends hand `c=`/`r=` to the terminal, the GUI backends multiply by their
  own cell metrics.
* `GraphicsLayer` — the set of placements owned by a `ScreenBuf`, reachable
  through `scr.Graphics()`. It is mutex protected, because decoding usually
  finishes on a worker goroutine while the UI thread is flushing.
* `GraphicsRenderer` — implemented by renderers that can display images.
  Renderers that cannot simply do not implement it.

## Protocol selection

`DetectGraphicsProtocol` reads the environment; `VTUI_GRAPHICS` overrides it
with `kitty`, `iterm2`, `sixel`, `native` or `none`. Inside `tmux` or `screen`
detection returns `none`, since a multiplexer that swallows half of an image
leaves the session in a mess. An application that can probe the terminal
should call `scr.Graphics().SetProtocol(...)` with the result.

## Redraw rules

Terminal graphics live above the cell grid, so an image has to be sent again
whenever the text below it was repainted — `GraphicsLayer.DirtyUnder` reports
exactly that, and the generation counter covers everything else. A forced
redraw (resize, `HardReset`, reattach) additionally tells the terminal to drop
every image it holds for us, otherwise stale placements would float over the
freshly painted screen.