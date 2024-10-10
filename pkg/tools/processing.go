package tools

import (
	"context"
	"iter"
	"slices"
	"sync"
)

func FanInSeqCtx[T any](ctx context.Context, inputs iter.Seq[<-chan T]) <-chan T {
	fanInFunc := func(ctx context.Context, wg *sync.WaitGroup, out chan<- T, in <-chan T) {
		defer wg.Done()
		// will keep working until the input channels are closed or the context is done
		for {
			select {
			case <-ctx.Done():
				return
			case alert, ok := <-in:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- alert:
				}
			}
		}
	}

	wg := new(sync.WaitGroup)
	out := make(chan T)
	for in := range inputs {
		wg.Add(1)
		go fanInFunc(ctx, wg, out, in)
	}

	go func() {
		defer close(out)
		wg.Wait()
	}()

	return out
}

func FanInSeq[T any](inputs iter.Seq[<-chan T]) <-chan T {
	return FanInSeqCtx(context.Background(), inputs)
}

func FanInCtx[T any](ctx context.Context, inputs ...<-chan T) <-chan T {
	return FanInSeqCtx(ctx, slices.Values(inputs))
}

func FanIn[T any](inputs ...<-chan T) <-chan T {
	return FanInCtx(context.Background(), inputs...)
}
