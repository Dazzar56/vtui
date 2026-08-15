# vreactive - Reactive Layer for vgui & vtui

`vreactive` provides a lightweight, thread-safe, and cycle-protected reactive primitives library for Go UI frameworks (`vtui` and `vgui`).

## Features

- **`Property[T]`**: Reactive state container with subscription handlers.
- **`Computed[T]` / `Computed2[T]`**: Automatically recomputed properties derived from one or two reactive dependencies.
- **`Bind[T]`**: One-way property synchronization.
- **`StateMachine`**: Declarative state transitions and property setters.
- **`Behavior[T]` & `Animator[T]`**: Smooth and discrete property transition animations (`SmoothBehavior`, `DiscreteBehavior`).
- **Cycle Detection**: Prevents infinite notification loops by enforcing a maximum call depth limit.
- **Thread Safety**: Mutex-protected reads/writes and `SafeSet` for asynchronous background goroutine updates via UI event queues.

## Usage Example

```go
nameProp := vreactive.NewProperty("Alice")
greetingProp := vreactive.Computed(nameProp, func(name string) string {
    return "Hello, " + name + "!"
})

// Reacting to changes
greetingProp.OnChange(func(val string) {
    fmt.Println(val)
})

nameProp.Set("Bob") // Prints "Hello, Bob!"
```
