package pricefeed

type Price struct {
	Symbol string
	USD    float64
}

type PriceProvider interface {
	UpdatePriceFromApi(priceCh chan<- []Price)
}
