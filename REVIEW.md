# Doubts and unfinished business in vtui

Decisions taken in a hurry, things noticed in passing, and things left alone
on purpose. One file, so a review can be one sitting.

## A gogpu drop is told nothing about the source

`OnDragDrop` hands over the paths and the position and nothing else: not the
actions the source allows, not the modifiers held at the drop, and there is
no channel back to the source to report what happened. So the backend
announces copy, always, and Shift or Ctrl over a gogpu window do nothing.
The modifiers we do pass on are the ones our own key handling last saw,
which during a drag from another application is usually nothing at all. If
gogpu ever grows a drag-over callback, this is the first thing to revisit.

## The drop position is trusted to be in the same pixels as the pointer

gogpu documents the drop position as physical pixels; its pointer callbacks
do not say. The host divides both by the same cell size, so on a HiDPI
window a drop would land in the wrong cell exactly when a click does.
Correcting it is a change to the coordinate handling of the whole host,
not to drag and drop, so it was deliberately left alone.

## Every drop costs two waits for the UI thread

The enter and the drop are delivered separately, and each gives the UI
thread up to `DragDeliverTimeout`. A target that answers at once, which is
the only one we have, never notices. A target that blocks turns half a
second of silence into a drop that did nothing.