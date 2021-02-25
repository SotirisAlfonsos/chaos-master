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

type dataSet struct {
	message           string
	inputItems        []item
	actionItem        *Key
	expectedItemCount int
}

type item struct {
	key *Key
	val func() (*v1.StatusResponse, error)
}

func TestShouldRegisterNewItem(t *testing.T) {
	testDataSet := []dataSet{
		{
			message: "Should register single item in the cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
			},
			expectedItemCount: 1,
		},
		{
			message: "Should register multiple items in the cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job 2", Target: "target 2"}, val: getSampleFunction()},
			},
			expectedItemCount: 2,
		},
		{
			message: "Should replace an existing item in the cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
			},
			expectedItemCount: 1,
		},
	}
	for _, testData := range testDataSet {
		t.Run(testData.message, func(t *testing.T) {
			manager := NewCacheManager(logger)

			for _, item := range testData.inputItems {
				if err := manager.Register(item.key, item.val); err != nil {
					t.Fatal("Should be able to add function to cache")
				}
			}

			assert.Equal(t, testData.expectedItemCount, manager.ItemCount())
		})
	}
}

func TestDeleteItem(t *testing.T) {
	testDataSet := []dataSet{
		{
			message: "Should delete single item for job that exists multiple times for different targets",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			actionItem:        &Key{Job: "job", Target: "target"},
			expectedItemCount: 1,
		},
		{
			message: "Should not delete anything if item does not exist in the cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			actionItem:        &Key{Job: "job 2", Target: "target"},
			expectedItemCount: 2,
		},
		{
			message:           "Should not delete anything if cache empty",
			inputItems:        []item{},
			actionItem:        &Key{Job: "job 2", Target: "target"},
			expectedItemCount: 0,
		},
	}
	for _, testData := range testDataSet {
		t.Run(testData.message, func(t *testing.T) {
			manager := NewCacheManager(logger)

			for _, item := range testData.inputItems {
				if err := manager.Register(item.key, item.val); err != nil {
					t.Fatal("Should be able to add function to cache")
				}
			}

			manager.Delete(testData.actionItem)

			assert.Equal(t, testData.expectedItemCount, manager.ItemCount())
		})
	}
}

func TestGetAllExistingItems(t *testing.T) {
	testDataSet := []dataSet{
		{
			message: "Should get all item from cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			expectedItemCount: 2,
		},
		{
			message:           "Should not get any items if cache is empty",
			inputItems:        []item{},
			expectedItemCount: 0,
		},
	}
	for _, testData := range testDataSet {
		t.Run(testData.message, func(t *testing.T) {
			manager := NewCacheManager(logger)

			for _, item := range testData.inputItems {
				if err := manager.Register(item.key, item.val); err != nil {
					t.Fatal("Should be able to add function to cache")
				}
			}

			cacheRetrievedItems := manager.GetAll()

			assert.Equal(t, testData.expectedItemCount, len(cacheRetrievedItems))
			for _, inputItem := range testData.inputItems {
				if _, ok := cacheRetrievedItems[*inputItem.key]; !ok {
					t.Fatal("Item should be retrieved from cache")
				}
			}
		})
	}
}

func TestGetExistingItem(t *testing.T) {
	testDataSet := []dataSet{
		{
			message: "Should get single item for job that exists multiple times for different targets",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			actionItem:        &Key{Job: "job", Target: "target"},
			expectedItemCount: 2,
		},
	}
	for _, testData := range testDataSet {
		t.Run(testData.message, func(t *testing.T) {
			manager := NewCacheManager(logger)

			for _, item := range testData.inputItems {
				if err := manager.Register(item.key, item.val); err != nil {
					t.Fatal("Should be able to add function to cache")
				}
			}

			function := manager.Get(testData.actionItem)

			assert.Equal(t, testData.expectedItemCount, manager.ItemCount())
			assert.NotNil(t, function)
		})
	}
}

func TestGetNonExistingItem(t *testing.T) {
	testDataSet := []dataSet{
		{
			message: "Should not get item for job that does not exist in cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			actionItem:        &Key{Job: "job 2", Target: "target"},
			expectedItemCount: 2,
		},
		{
			message: "Should not get item for target that does not exist in cache",
			inputItems: []item{
				{key: &Key{Job: "job", Target: "target"}, val: getSampleFunction()},
				{key: &Key{Job: "job", Target: "target 2"}, val: getSampleFunction()},
			},
			actionItem:        &Key{Job: "job", Target: "target 3"},
			expectedItemCount: 2,
		},
		{
			message:           "Should not get item from empty cache",
			inputItems:        []item{},
			actionItem:        &Key{Job: "job", Target: "target 3"},
			expectedItemCount: 0,
		},
	}
	for _, testData := range testDataSet {
		t.Run(testData.message, func(t *testing.T) {
			manager := NewCacheManager(logger)

			for _, item := range testData.inputItems {
				if err := manager.Register(item.key, item.val); err != nil {
					t.Fatal("Should be able to add function to cache")
				}
			}

			function := manager.Get(testData.actionItem)

			assert.Equal(t, testData.expectedItemCount, manager.ItemCount())
			assert.Nil(t, function)
		})
	}
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
