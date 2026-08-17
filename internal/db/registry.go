package db

import "fmt"

type Factory func() Driver

var registry = map[DbType]Factory{}

func Register(dbType DbType, f Factory) {
	registry[dbType] = f
}

func NewDriver(dbType DbType) (Driver, error) {
	f, ok := registry[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported db type: %s", dbType)
	}
	return f(), nil
}
