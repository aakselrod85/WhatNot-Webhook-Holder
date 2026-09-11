package entity

type SeriesPriceRange struct {
	Id        int64 `json:"id"         db:"id"`
	SeriesId  int64 `json:"series_id"  db:"series_id"`
	PriceFrom int   `json:"price_from" db:"price_from"`
	PriceTo   *int  `json:"price_to"   db:"price_to"` // nil = open-ended
	Count     int   `json:"count"      db:"count"`
}
