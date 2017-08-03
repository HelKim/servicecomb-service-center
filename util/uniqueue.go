//Copyright 2017 Huawei Technologies Co., Ltd
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
package util

import (
	"errors"
	"golang.org/x/net/context"
	"math"
	"sync"
)

const DEFAULT_MAX_BUFFER_SIZE = 1000

type UniQueue struct {
	size   int
	buffer chan interface{}
	queue  chan interface{}
	close  chan struct{}
	lock   sync.RWMutex
	once   sync.Once
}

func (uq *UniQueue) Get(ctx context.Context) interface{} {
	select {
	case <-uq.close:
		return nil
	case <-ctx.Done():
		return nil
	case item := <-uq.queue:
		return item
	}
}

func (uq *UniQueue) Put(ctx context.Context, value interface{}) error {
	uq.once.Do(func() {
		go uq.do()
	})
	uq.lock.RLock()
	select {
	case <-uq.close:
		uq.lock.RUnlock()
		return errors.New("channel is closed")
	default:
		select {
		case <-ctx.Done():
			uq.lock.RUnlock()
			return errors.New("timed out")
		case uq.buffer <- value:
		}
	}
	uq.lock.RUnlock()
	return nil
}

func (uq *UniQueue) do() {
	for {
		select {
		case item, ok := <-uq.buffer:
			if !ok {
				return
			}
			select {
			case _, ok := <-uq.queue:
				if !ok {
					return
				}
				uq.sendToQueue(item)
			default:
				uq.sendToQueue(item)
			}
		}
	}
}

func (uq *UniQueue) sendToQueue(item interface{}) {
	uq.lock.RLock()
	select {
	case <-uq.close:
		uq.lock.RUnlock()
		return
	default:
		select {
		case uq.queue <- item:
		default:
		}
	}
	uq.lock.RUnlock()
}

func (uq *UniQueue) Close() {
	select {
	case <-uq.close:
	default:
		uq.lock.Lock()
		close(uq.close)
		close(uq.queue)
		close(uq.buffer)
		uq.lock.Unlock()
	}
}

func newUniQueue(size int) (*UniQueue, error) {
	if size <= 0 || size >= math.MaxInt32 {
		return nil, errors.New("invalid buffer size")
	}
	return &UniQueue{
		size:   size,
		queue:  make(chan interface{}, 1),
		buffer: make(chan interface{}, size),
		close:  make(chan struct{}),
	}, nil
}

func NewUniQueue() (uq *UniQueue) {
	uq, _ = newUniQueue(DEFAULT_MAX_BUFFER_SIZE)
	return
}
