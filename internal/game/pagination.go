package game

import "errors"

const MaxPageSize = 100

// ListOptions requests an ascending ID range. Limit may include one look-ahead row.
type ListOptions struct {
	AfterID int64
	Limit   int
}

func (o ListOptions) Validate() error {
	if o.AfterID < 0 || o.Limit < 1 || o.Limit > MaxPageSize+1 {
		return errors.New("invalid catalog list options")
	}
	return nil
}
