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
    "fmt"
    "testing"
)

type Tk struct {
}

func (tk *Tk) TrieNodeWalker(b string) {
    fmt.Printf("%s\n", b)
}

func (tk *Tk) TrieData2String(d TrieData) string {
    
    if data, ok := d.(int); ok {
        return fmt.Sprintf("%d", data)
    }

    return ""
}

func BenchmarkTrie(b *testing.B) {
    var tk Tk;
    trieR := TrieInit()

    i := 0
    j := 0
    k := 0
    pLen := 32

    for n := 0; n < b.N; n++ {
        i = n & 0xff
        j = n >> 8 & 0xff
        k = n >> 16 & 0xff

        /*if j > 0 {
            pLen = 24
        } else {
            pLen = 32
        }*/
        route := fmt.Sprintf("192.%d.%d.%d/%d", k, j, i, pLen)
        res := trieR.AddTrie(route, n)
        if res != 0 {
            b.Errorf("failed to add %s:%d - (%d)", route, n, res)
            trieR.Trie2String(&tk)
        }
    }
}

func TestTrie(t *testing.T) {
    trieR := TrieInit()
    route := "192.168.1.1/32"
    data := 1100
    res := trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "192.168.1.0/15"
    data = 100
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "192.168.1.0/16"
    data = 99
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "192.168.1.0/8"
    data = 1
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "192.168.1.0/16"
    data = 1
    res = trieR.AddTrie(route, data)
    if res == 0 {
        t.Errorf("re-added %s:%d", route, data)
    }

    route = "0.0.0.0/0"
    data = 222
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "8.8.8.8/32"
    data = 1200
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "10.10.10.10/32"
    data = 12
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    route = "1.1.1.1/32"
    data = 1212
    res = trieR.AddTrie(route, data)
    if res != 0 {
        t.Errorf("failed to add %s:%d", route, data)
    }

    ret, pfx, rdata := trieR.FindTrie("192.41.3.1")
    if ret != 0 || *pfx != "192.0.0.0/8" || rdata != 1 {
        t.Errorf("failed to find %s", "192.41.3.1")
    }

    ret1, pfx1, rdata1 := trieR.FindTrie("195.41.3.1")
    if ret1 != 0 || *pfx1 != "0.0.0.0/0" || rdata1 != 222 {
        t.Errorf("failed to find %s", "195.41.3.1")
    }

    ret2, pfx2, rdata2 := trieR.FindTrie("8.8.8.8")
    if ret2 != 0 || *pfx2 != "8.8.8.8/32" || rdata2 != 1200 {
        t.Errorf("failed to find %d %s %d", ret, "8.8.8.8", rdata)
    }

    route = "0.0.0.0/0"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    ret1, pfx1, rdata1 = trieR.FindTrie("195.41.3.1")
    if ret1 == 0 {
        t.Errorf("failed to find %s", "195.41.3.1")
    }

    route = "192.168.1.1/32"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "192.168.1.0/15"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "192.168.1.0/16"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "192.168.1.0/8"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "0.0.0.0/0"
    res = trieR.DelTrie(route)
    if res == 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "8.8.8.8/32"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "10.10.10.10/32"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

    route = "1.1.1.1/24"
    res = trieR.DelTrie(route)
    if res != 0 {
        t.Errorf("failed to delete %s", route)
    }

}

func TestCounter(t *testing.T) {

    cR := NewCounter(0, 10)

    for i := 0; i < 12; i++ {
        idx, err := cR.GetCounter()

        if err == nil {
            if i > 9 {
                t.Errorf("get Counter unexpected %d:%d", i, idx)
            }
        } else {
            if i <= 9 {
                t.Errorf("failed to get Counter %d:%d", i, idx)
            }
        }
    }

    err := cR.PutCounter(5)
    if err != nil {
        t.Errorf("failed to put valid Counter %d", 5)
    }

    err = cR.PutCounter(2)
    if err != nil {
        t.Errorf("failed to put valid Counter %d", 2)
    }

    err = cR.PutCounter(15)
    if err == nil {
        t.Errorf("Able to put invalid Counter %d", 15)
    }

    var idx int
    idx, err = cR.GetCounter()
    if idx != 5 || err != nil {
        t.Errorf("Counter get got %d of expected %d", idx, 5)
    }

    idx, err = cR.GetCounter()
    if idx != 2 || err != nil {
        t.Errorf("Counter get got %d of expected %d", idx, 2)
    }
}
