package main

import (
	"bytes"
	"encoding/gob"
	"fmt"

	. "github.com/zxfonline/net/packet"
)

func GobEncode(v interface{}) []byte {
	buffer := new(bytes.Buffer)
	enc := gob.NewEncoder(buffer)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

func GobDecode(v interface{}, data []byte) {
	enc := gob.NewDecoder(bytes.NewReader(data))
	if err := enc.Decode(v); err != nil {
		panic(err)
	}
}

func main() {
	//	n1 := GobEncode(&P{3, 4, 5, "Pythagoras"})
	n1 := GobEncode(P{3, 4, 5, "Pythagoras"})

	q := new(Q)
	GobDecode(q, n1)
	fmt.Printf("%+v: {%d,%d}\n", q.Name, *q.X, *q.Y)

	fmt.Println("---------")
	bb := NewBuffer(1, nil)
	//	bb.WriteBinaryData(&P{3, 4, 5, "Pythagoras"})
	bb.WriteBinaryData(P{3, 4, 5, "Pythagoras"}, false)
	q2 := new(Q)
	fmt.Printf("%p\n", q2)
	bb.ReadBinaryData(q2, false)
	fmt.Printf("%+v: {%d,%d}\n", q2.Name, *q2.X, *q2.Y)
	fmt.Printf("%p\n", q2)

	fmt.Println("---------")
	arr := []int16{1, 2, 3, 4, 5, 6}
	bb.WriteBinaryData(arr, false)
	var arr1 []int16
	fmt.Printf("%p\n", arr1)
	bb.ReadBinaryData(&arr1, false)
	fmt.Printf("%v %v %v\n", arr1, len(arr1), cap(arr1))
	fmt.Printf("%p\n", arr1)
	fmt.Println("---------")
	arr2 := make(map[int]string)
	arr2[1] = "1"
	arr2[2] = "2"
	bb.WriteBinaryData(arr2, false)
	arr21 := make(map[int]string)
	fmt.Printf("%p\n", arr21)
	bb.ReadBinaryData(&arr21, false)
	fmt.Printf("%v %v\n", arr21, len(arr21))
	fmt.Printf("%p\n", arr21)
	fmt.Println("---------")
	TestPack()
}

type P struct {
	X, Y, Z int
	Name    string
}

type Q struct {
	X, Y *int32
	Name string
}

type SUB2 struct {
	M []int16
}

type SUB struct {
	H int16
	I uint16
}
type TEST struct {
	BOOL bool
	A    int32
	B    string
	C    float32
	D    uint32
	E    float64
	F    []byte
	Sub  []SUB
	S2   SUB
	S3   SUB2
}

func TestPack() {
	arr21 := make(map[int]string)
	fmt.Printf("%p\n", arr21)
	fmt.Printf("%v %v\n", arr21, len(arr21))
	fmt.Printf("%p\n", arr21)

	test := TEST{BOOL: true, A: 16, B: string([]byte{65}), C: 1.0, D: 32, E: 1.0, F: []byte{1, 2, 3, 4, 5}}
	test.Sub = make([]SUB, 2)
	test.Sub[0].H = 1024
	test.Sub[0].I = 2048
	test.Sub[1].H = 4096
	test.Sub[1].I = 8192
	test.S2.H = 100
	test.S3.M = make([]int16, 10)
	bb := NewBuffer(1, nil)
	bb.WriteBinaryData(test, false)
	test1 := new(TEST)
	bb.ReadBinaryData(test1, false)
	fmt.Printf("%+v\n", test1)
}
