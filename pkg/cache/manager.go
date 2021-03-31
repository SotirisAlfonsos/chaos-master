package cache

import (
	"reflect"

	"github.com/SotirisAlfonsos/gocache"
)

type Key struct {
	Job    string
	Target string
}

func (k Key) Equals(key gocache.Key) bool {
	if key == nil || reflect.ValueOf(key).Kind() == reflect.Ptr {
		return false
	}

	return k.Job == key.(Key).Job && k.Target == key.(Key).Target
}
