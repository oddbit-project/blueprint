package db

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"slices"
)

const (
	SortAscending  = "asc"
	SortDescending = "desc"

	SearchNone  = 0
	SearchStart = 1
	SearchEnd   = 2
	SearchAny   = 3

	DefaultPageSize = 100
)

type GridFilterFunc func(lookupValue any) (any, error)

type Grid struct {
	tableName  string
	spec       *FieldSpec
	filterFunc map[string]GridFilterFunc // filtering functions to translate GridQuery filter values to db values, eg: field:yes -> field:true
}

type GridQuery struct {
	SearchType   uint              `db:"searchType"`
	SearchText   string            `json:"searchText,omitempty"`
	FilterFields map[string]any    `json:"filterFields,omitEmpty"`
	SortFields   map[string]string `json:"sortFields,omitempty"`
	Offset       uint              `json:"offset,omitempty"`
	Limit        uint              `json:"limit,omitempty"`
}

type GridError struct {
	Scope   string `json:"scope"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

var (
	validSearchType = []uint{SearchNone, SearchStart, SearchEnd, SearchAny}
	validSortFields = []string{SortDescending, SortAscending}
)

// error interface
func (err GridError) Error() string {
	if err.Field != "" {
		return fmt.Sprintf("error on %s with field %s: %s", err.Scope, err.Field, err.Message)
	}
	return fmt.Sprintf("error on %s: %s", err.Scope, err.Message)
}

// NewGridQuery helper to create a GridQuery
func NewGridQuery(searchType uint, limit uint, offset uint) (GridQuery, error) {
	if slices.Index(validSearchType, searchType) < 0 {
		return GridQuery{}, GridError{
			Scope:   "search",
			Field:   "",
			Message: "invalid search type",
		}
	}
	return GridQuery{
		SearchType:   searchType,
		SearchText:   "",
		FilterFields: nil,
		SortFields:   nil,
		Offset:       offset,
		Limit:        limit,
	}, nil
}

// Page calculates offset and limit from page information
func (g GridQuery) Page(page, itemsPerPage int) {
	if page < 1 {
		page = 1
	}
	if itemsPerPage < 1 {
		itemsPerPage = DefaultPageSize
	}

	g.Offset = uint((page - 1) * itemsPerPage)
	g.Limit = uint(itemsPerPage)
}

// NewGrid create a new grid
func NewGrid(tableName string, record any) (*Grid, error) {
	spec, err := NewFieldSpec(record)
	if err != nil {
		return nil, err
	}
	return NewGridWithSpec(tableName, spec), nil
}

func NewGridWithSpec(tableName string, spec *FieldSpec) *Grid {
	return &Grid{
		tableName:  tableName,
		spec:       spec,
		filterFunc: make(map[string]GridFilterFunc),
	}
}

// AddFilterFunc register a new filtering function
// filtering functions translate filtering values to db-compatible values
func (grid *Grid) AddFilterFunc(dbField string, f GridFilterFunc) *Grid {
	grid.filterFunc[dbField] = f
	return grid
}

// ValidQuery validates if a GridQuery request is valid
func (grid *Grid) ValidQuery(query GridQuery) error {
	// match filterable fields
	if query.FilterFields != nil {
		for f := range query.FilterFields {
			fname, ok := grid.spec.LookupAlias(f)
			if !ok {
				return GridError{
					Scope:   "filter",
					Field:   f,
					Message: "field is not valid",
				}
			}
			// lookup db field to see if its filterable
			if slices.Index(grid.spec.FilterFields(), fname) < 0 {
				return GridError{
					Scope:   "filter",
					Field:   f,
					Message: "field is not filterable",
				}
			}

			// validate filter func
			if fn, ok := grid.filterFunc[f]; ok {
				if _, err := fn(query.FilterFields[f]); err != nil {
					return err
				}
			}
		}
	}

	// match sortable fields
	if query.SortFields != nil {
		for f, v := range query.SortFields {
			fname, ok := grid.spec.LookupAlias(f)
			if !ok {
				return GridError{
					Scope:   "sort",
					Field:   f,
					Message: "field is not valid",
				}
			}
			// lookup db field to see if its sortable
			if slices.Index(grid.spec.SortFields(), fname) < 0 {
				return GridError{
					Scope:   "sort",
					Field:   f,
					Message: "field is not sortable",
				}
			}
			if len(v) > 0 {
				if slices.Index(validSortFields, v) < 0 {
					return GridError{
						Scope:   "sort",
						Field:   f,
						Message: "sort order is not valid",
					}
				}
			}
		}
	}

	if len(query.SearchText) > 0 && query.SearchType == SearchNone {
		return GridError{
			Scope:   "search",
			Field:   "",
			Message: "search not allowed",
		}
	}

	return nil
}

func (grid *Grid) Build(qry *goqu.SelectDataset, args GridQuery) (*goqu.SelectDataset, error) {
	if qry == nil {
		qry = goqu.Select().From(grid.tableName)
	}

	// process filters
	if args.FilterFields != nil {
		for f, v := range args.FilterFields {
			fname, ok := grid.spec.LookupAlias(f)
			if !ok {
				return nil, GridError{
					Scope:   "filter",
					Field:   f,
					Message: "field is not valid",
				}
			}
			// lookup db field to see if its filterable
			if slices.Index(grid.spec.FilterFields(), fname) < 0 {
				return nil, GridError{
					Scope:   "filter",
					Field:   f,
					Message: "field is not filterable",
				}
			}

			// apply filter
			// first look for any filter functions
			if fn, valid := grid.filterFunc[fname]; valid {
				nv, err := fn(v)
				if err != nil {
					return nil, err
				}
				qry = qry.Where(goqu.Ex{fname: nv})
			} else {
				qry = qry.Where(goqu.Ex{fname: v})
			}

		}
	}

	// process search
	if len(args.SearchText) > 0 {
		searchExpr := ""
		switch args.SearchType {
		case SearchNone:
			return nil, GridError{
				Scope:   "search",
				Field:   "",
				Message: "search not allowed",
			}
		case SearchStart:
			searchExpr = "%" + args.SearchText
		case SearchEnd:
			searchExpr = args.SearchText + "%"
		case SearchAny:
			searchExpr = "%" + args.SearchText + "%"
		default:
			return nil, GridError{
				Scope:   "search",
				Field:   "",
				Message: "invalid search type",
			}
		}

		searchFields := grid.spec.SearchFields()
		expr := make([]goqu.Expression, len(searchFields))
		for i, field := range searchFields {
			expr[i] = goqu.I(field).Like(searchExpr)
		}
		qry = qry.Where(goqu.Or(expr...))
	}

	// process sorts
	if args.SortFields != nil {
		for f, v := range args.SortFields {
			sorting := SortDescending
			if len(v) > 0 {
				sorting = v
			}
			fname, ok := grid.spec.LookupAlias(f)
			if !ok {
				return nil, GridError{
					Scope:   "sort",
					Field:   f,
					Message: "field is not valid",
				}
			}
			// lookup db field to see if its sortable
			if slices.Index(grid.spec.SortFields(), fname) < 0 {
				return nil, GridError{
					Scope:   "sort",
					Field:   f,
					Message: "field is not sortable",
				}
			}

			// check sort direction
			if slices.Index(validSortFields, sorting) < 0 {
				return nil, GridError{
					Scope:   "sort",
					Field:   f,
					Message: "sort order is not valid",
				}
			}

			// apply sort
			if sorting == SortAscending {
				qry = qry.OrderAppend(goqu.I(fname).Asc())
			} else {
				qry = qry.OrderAppend(goqu.I(fname).Desc())
			}
		}
	}

	//offset & limit
	if args.Offset >= 0 {
		if args.Limit > 0 {
			// offset 0 only makes sense if limit is set
			qry = qry.Offset(args.Offset).Limit(args.Limit)
		} else {
			if args.Offset > 0 {
				qry = qry.Offset(args.Offset)
			}
		}
	}

	return qry, nil
}
