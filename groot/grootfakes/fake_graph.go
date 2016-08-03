// This file was generated by counterfeiter
package grootfakes

import (
	"sync"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type FakeGraph struct {
	MakeBundleStub        func(lager.Logger, string) (groot.Bundle, error)
	makeBundleMutex       sync.RWMutex
	makeBundleArgsForCall []struct {
		arg1 lager.Logger
		arg2 string
	}
	makeBundleReturns struct {
		result1 groot.Bundle
		result2 error
	}
	DeleteBundleStub        func(logger lager.Logger, id string) error
	deleteBundleMutex       sync.RWMutex
	deleteBundleArgsForCall []struct {
		logger lager.Logger
		id     string
	}
	deleteBundleReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeGraph) MakeBundle(arg1 lager.Logger, arg2 string) (groot.Bundle, error) {
	fake.makeBundleMutex.Lock()
	fake.makeBundleArgsForCall = append(fake.makeBundleArgsForCall, struct {
		arg1 lager.Logger
		arg2 string
	}{arg1, arg2})
	fake.recordInvocation("MakeBundle", []interface{}{arg1, arg2})
	fake.makeBundleMutex.Unlock()
	if fake.MakeBundleStub != nil {
		return fake.MakeBundleStub(arg1, arg2)
	} else {
		return fake.makeBundleReturns.result1, fake.makeBundleReturns.result2
	}
}

func (fake *FakeGraph) MakeBundleCallCount() int {
	fake.makeBundleMutex.RLock()
	defer fake.makeBundleMutex.RUnlock()
	return len(fake.makeBundleArgsForCall)
}

func (fake *FakeGraph) MakeBundleArgsForCall(i int) (lager.Logger, string) {
	fake.makeBundleMutex.RLock()
	defer fake.makeBundleMutex.RUnlock()
	return fake.makeBundleArgsForCall[i].arg1, fake.makeBundleArgsForCall[i].arg2
}

func (fake *FakeGraph) MakeBundleReturns(result1 groot.Bundle, result2 error) {
	fake.MakeBundleStub = nil
	fake.makeBundleReturns = struct {
		result1 groot.Bundle
		result2 error
	}{result1, result2}
}

func (fake *FakeGraph) DeleteBundle(logger lager.Logger, id string) error {
	fake.deleteBundleMutex.Lock()
	fake.deleteBundleArgsForCall = append(fake.deleteBundleArgsForCall, struct {
		logger lager.Logger
		id     string
	}{logger, id})
	fake.recordInvocation("DeleteBundle", []interface{}{logger, id})
	fake.deleteBundleMutex.Unlock()
	if fake.DeleteBundleStub != nil {
		return fake.DeleteBundleStub(logger, id)
	} else {
		return fake.deleteBundleReturns.result1
	}
}

func (fake *FakeGraph) DeleteBundleCallCount() int {
	fake.deleteBundleMutex.RLock()
	defer fake.deleteBundleMutex.RUnlock()
	return len(fake.deleteBundleArgsForCall)
}

func (fake *FakeGraph) DeleteBundleArgsForCall(i int) (lager.Logger, string) {
	fake.deleteBundleMutex.RLock()
	defer fake.deleteBundleMutex.RUnlock()
	return fake.deleteBundleArgsForCall[i].logger, fake.deleteBundleArgsForCall[i].id
}

func (fake *FakeGraph) DeleteBundleReturns(result1 error) {
	fake.DeleteBundleStub = nil
	fake.deleteBundleReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeGraph) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.makeBundleMutex.RLock()
	defer fake.makeBundleMutex.RUnlock()
	fake.deleteBundleMutex.RLock()
	defer fake.deleteBundleMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeGraph) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ groot.Graph = new(FakeGraph)
