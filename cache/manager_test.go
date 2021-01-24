package cache

import (
	"fmt"
	"testing"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

var (
	logger = getLogger()
)

func TestShouldRegisterNewItem(t *testing.T) {
	manager := NewCacheManager(logger)

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	if err := manager.Register("test 2", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	assert.Equal(t, 2, manager.ItemCount())
}

func TestShouldReplaceExistingItem(t *testing.T) {
	manager := NewCacheManager(logger)

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	assert.Equal(t, 1, manager.ItemCount())
}

func TestShouldDeleteItem(t *testing.T) {
	manager := NewCacheManager(logger)

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	manager.Delete("test")

	assert.Equal(t, 0, manager.ItemCount())
}

func TestGetExistingItem(t *testing.T) {
	manager := NewCacheManager(logger)

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	function := manager.Get("test")

	assert.Equal(t, 1, manager.ItemCount())
	assert.NotNil(t, function)
}

func TestGetNilForNonExistingItem(t *testing.T) {
	manager := NewCacheManager(logger)

	if err := manager.Register("test", getSampleFunction()); err != nil {
		t.Fatal("Should be able to add function to cache")
	}

	function := manager.Get("test 2")

	assert.Equal(t, 1, manager.ItemCount())
	assert.Nil(t, function)
}

func getSampleFunction() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{}, nil
	}
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
