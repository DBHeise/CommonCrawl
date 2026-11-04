package CommonCrawl

type Handler[TIn any] func(int, TIn)
type Handler2[TIn any, TOut any] func(int, TIn, chan TOut)
