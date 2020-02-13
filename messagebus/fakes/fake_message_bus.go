// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"crypto/tls"
	"sync"

	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/route-registrar/messagebus"
)

type FakeMessageBus struct {
	CloseStub        func()
	closeMutex       sync.RWMutex
	closeArgsForCall []struct {
	}
	ConnectStub        func([]config.MessageBusServer, *tls.Config) error
	connectMutex       sync.RWMutex
	connectArgsForCall []struct {
		arg1 []config.MessageBusServer
		arg2 *tls.Config
	}
	connectReturns struct {
		result1 error
	}
	connectReturnsOnCall map[int]struct {
		result1 error
	}
	SendMessageStub        func(string, string, config.Route, string) error
	sendMessageMutex       sync.RWMutex
	sendMessageArgsForCall []struct {
		arg1 string
		arg2 string
		arg3 config.Route
		arg4 string
	}
	sendMessageReturns struct {
		result1 error
	}
	sendMessageReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeMessageBus) Close() {
	fake.closeMutex.Lock()
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct {
	}{})
	fake.recordInvocation("Close", []interface{}{})
	fake.closeMutex.Unlock()
	if fake.CloseStub != nil {
		fake.CloseStub()
	}
}

func (fake *FakeMessageBus) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakeMessageBus) CloseCalls(stub func()) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = stub
}

func (fake *FakeMessageBus) Connect(arg1 []config.MessageBusServer, arg2 *tls.Config) error {
	var arg1Copy []config.MessageBusServer
	if arg1 != nil {
		arg1Copy = make([]config.MessageBusServer, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.connectMutex.Lock()
	ret, specificReturn := fake.connectReturnsOnCall[len(fake.connectArgsForCall)]
	fake.connectArgsForCall = append(fake.connectArgsForCall, struct {
		arg1 []config.MessageBusServer
		arg2 *tls.Config
	}{arg1Copy, arg2})
	fake.recordInvocation("Connect", []interface{}{arg1Copy, arg2})
	fake.connectMutex.Unlock()
	if fake.ConnectStub != nil {
		return fake.ConnectStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.connectReturns
	return fakeReturns.result1
}

func (fake *FakeMessageBus) ConnectCallCount() int {
	fake.connectMutex.RLock()
	defer fake.connectMutex.RUnlock()
	return len(fake.connectArgsForCall)
}

func (fake *FakeMessageBus) ConnectCalls(stub func([]config.MessageBusServer, *tls.Config) error) {
	fake.connectMutex.Lock()
	defer fake.connectMutex.Unlock()
	fake.ConnectStub = stub
}

func (fake *FakeMessageBus) ConnectArgsForCall(i int) ([]config.MessageBusServer, *tls.Config) {
	fake.connectMutex.RLock()
	defer fake.connectMutex.RUnlock()
	argsForCall := fake.connectArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeMessageBus) ConnectReturns(result1 error) {
	fake.connectMutex.Lock()
	defer fake.connectMutex.Unlock()
	fake.ConnectStub = nil
	fake.connectReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeMessageBus) ConnectReturnsOnCall(i int, result1 error) {
	fake.connectMutex.Lock()
	defer fake.connectMutex.Unlock()
	fake.ConnectStub = nil
	if fake.connectReturnsOnCall == nil {
		fake.connectReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.connectReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeMessageBus) SendMessage(arg1 string, arg2 string, arg3 config.Route, arg4 string) error {
	fake.sendMessageMutex.Lock()
	ret, specificReturn := fake.sendMessageReturnsOnCall[len(fake.sendMessageArgsForCall)]
	fake.sendMessageArgsForCall = append(fake.sendMessageArgsForCall, struct {
		arg1 string
		arg2 string
		arg3 config.Route
		arg4 string
	}{arg1, arg2, arg3, arg4})
	fake.recordInvocation("SendMessage", []interface{}{arg1, arg2, arg3, arg4})
	fake.sendMessageMutex.Unlock()
	if fake.SendMessageStub != nil {
		return fake.SendMessageStub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.sendMessageReturns
	return fakeReturns.result1
}

func (fake *FakeMessageBus) SendMessageCallCount() int {
	fake.sendMessageMutex.RLock()
	defer fake.sendMessageMutex.RUnlock()
	return len(fake.sendMessageArgsForCall)
}

func (fake *FakeMessageBus) SendMessageCalls(stub func(string, string, config.Route, string) error) {
	fake.sendMessageMutex.Lock()
	defer fake.sendMessageMutex.Unlock()
	fake.SendMessageStub = stub
}

func (fake *FakeMessageBus) SendMessageArgsForCall(i int) (string, string, config.Route, string) {
	fake.sendMessageMutex.RLock()
	defer fake.sendMessageMutex.RUnlock()
	argsForCall := fake.sendMessageArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeMessageBus) SendMessageReturns(result1 error) {
	fake.sendMessageMutex.Lock()
	defer fake.sendMessageMutex.Unlock()
	fake.SendMessageStub = nil
	fake.sendMessageReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeMessageBus) SendMessageReturnsOnCall(i int, result1 error) {
	fake.sendMessageMutex.Lock()
	defer fake.sendMessageMutex.Unlock()
	fake.SendMessageStub = nil
	if fake.sendMessageReturnsOnCall == nil {
		fake.sendMessageReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.sendMessageReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeMessageBus) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	fake.connectMutex.RLock()
	defer fake.connectMutex.RUnlock()
	fake.sendMessageMutex.RLock()
	defer fake.sendMessageMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeMessageBus) recordInvocation(key string, args []interface{}) {
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

var _ messagebus.MessageBus = new(FakeMessageBus)
