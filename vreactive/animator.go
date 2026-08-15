package vreactive

type DiscreteAnimator[T any] struct {
	Target T
}

func (a *DiscreteAnimator[T]) Tick(dt float64) (T, bool) {
	return a.Target, true
}

type DiscreteBehavior[T any] struct{}

func (b *DiscreteBehavior[T]) CreateAnimator(start, end T) Animator[T] {
	return &DiscreteAnimator[T]{Target: end}
}

type Interpolator[T any] func(start, end T, progress float64) T

type SmoothAnimator[T any] struct {
	Start    T
	End      T
	Duration float64
	Elapsed  float64
	Interp   Interpolator[T]
}

func (a *SmoothAnimator[T]) Tick(dt float64) (T, bool) {
	a.Elapsed += dt
	if a.Elapsed >= a.Duration {
		return a.End, true
	}
	return a.Interp(a.Start, a.End, a.Elapsed/a.Duration), false
}

// SmoothBehavior smoothly interpolates property transitions over Duration.
type SmoothBehavior[T any] struct {
	Duration float64
	Interp   Interpolator[T]
}

func (b *SmoothBehavior[T]) CreateAnimator(start, end T) Animator[T] {
	return &SmoothAnimator[T]{
		Start:    start,
		End:      end,
		Duration: b.Duration,
		Interp:   b.Interp,
	}
}

func Float64Interpolator(start, end float64, progress float64) float64 {
	return start + (end-start)*progress
}

func IntInterpolator(start, end int, progress float64) int {
	return start + int(float64(end-start)*progress)
}
