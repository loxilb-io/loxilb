/*
 * Copyright (c) 2022 NetLOX Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package loxilib

import (
	"errors"
)

type Counter struct {
	begin    int
	end      int
	start    int
	len      int
	cap      int
	counters []int
}

func NewCounter(begin int, length int) *Counter {
	counter := new(Counter)
	counter.counters = make([]int, length)
	counter.begin = begin
	counter.start = 0
	counter.end = length - 1
	counter.len = length
	counter.cap = length
	for i := 0; i < length; i++ {
		counter.counters[i] = i + 1
	}
	//fmt.Println(counter.counters)
	counter.counters[length-1] = -1
	return counter
}

func (C *Counter) Get_counter_p() (int, error) {
	if C.start == -1 || C.cap <= 0 {
		return -1, errors.New("Overflow")
	}

	var rid = C.start
	C.start = C.counters[rid]
	C.cap--

	return rid + C.begin, nil
}

func (C *Counter) Put_counter_p(id int) error {
	if id < C.begin || id >= C.begin+C.len || C.counters[id] == -1 {
		return errors.New("Range")
	}
	var tmp = C.start
	C.start = id
	C.counters[id] = tmp
	C.cap++

	return nil
}

func (C *Counter) GetCounter() (int, error) {
	if C.cap <= 0 || C.start == -1 {
		return -1, errors.New("Overflow")
	}

	C.cap--
	var rid = C.start
	if C.start == C.end {
		C.start = -1
	} else {
		C.start = C.counters[rid]
		C.counters[rid] = -1
	}
	return rid + C.begin, nil
}

func (C *Counter) PutCounter(id int) error {
	if id < C.begin || id >= C.begin+C.len {
		return errors.New("Range")
	}
	rid := id - C.begin
	var tmp = C.end
	C.end = rid
	C.counters[tmp] = rid
	C.cap++
	if C.start == -1 {
		C.start = C.end
	}
	return nil
}
