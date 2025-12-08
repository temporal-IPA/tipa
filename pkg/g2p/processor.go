package g2p

import "context"

// Processor is the minimal building block for the g2p pipeline.
//
// A Processor takes an existing phonetic Result and returns a new Result.
// Implementations are free to:
//
//   - add new Fragments;
//   - modify existing Fragments;
//   - shrink or refine RawTexts;
//
// but they must preserve Result.Text and keep Pos / Len expressed in
// rune offsets in that text.
type Processor interface {
	Apply(input Result) Result
}

// CancellableProcessor is the streaming / cancellable counterpart of
// Processor.
//
// Implementations typically emit a single Result on the returned
// channel, but they are allowed to emit more than one if it makes
// sense for them.
//
// The channel must be closed in all cases, including when ctx is canceled.
type CancellableProcessor interface {
	StreamApply(ctx context.Context, input Result) <-chan Result
}
