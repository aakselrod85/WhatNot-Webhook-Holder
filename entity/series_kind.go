package entity

const (
	SeriesKindCards       = "cards"
	SeriesKindPriceRanges = "price_ranges"
)

func IsValidSeriesKind(kind string) bool {
	return kind == SeriesKindCards || kind == SeriesKindPriceRanges
}
