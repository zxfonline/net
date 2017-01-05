// Copyright 2016 zxfonline@sina.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"

	//	"github.com/golang/protobuf/proto"
	"github.com/ugorji/go/codec"
	"github.com/zxfonline/buffpool"
)

//将 v 编码写入到buffer中 head:是否有消息体长度 (eg:"encoding/gob","github.com/golang/protobuf/proto","http://msgpack.org/")
type Serialization func(nb IoBuffer, v interface{}, head bool)

//读取一个data解码到v(a pointer interface{}) head:是否有消息体长度
type DeSerialization func(nb IoBuffer, v interface{}, head bool)

var (
	//默认对象序列化
	Default_Serialization Serialization
	//默认对象反序列化
	Default_DeSerialization DeSerialization
	//默认字节序 大端法
	DefaultEndian = binary.BigEndian
)

func init() {
	////	InitSerialization(WriteProtoData, ReadProtoData)
	//	InitSerialization(WriteGobData, ReadGobData)
	InitSerialization(WriteMsgpData, ReadMsgpData)
}

////将 v 通过 "github.com/golang/protobuf/proto" 编码写入到buffer中 head:是否有消息体长度
//var WriteProtoData = func(nb IoBuffer, v proto.Message, head bool) IoBuffer {
//	if bb, err := proto.Marshal(v); err == nil {
//		if head {
//			nb.WriteDataWithHead(bb)
//		} else {
//			nb.WriteData(bb)
//		}
//	} else {
//		panic(err)
//	}
//	return nb
//}

////读取一个data通过 "github.com/golang/protobuf/proto" 解码到v(a pointer proto.Message) head:是否有消息体长度
//var ReadProtoData = func(nb IoBuffer, v proto.Message, head bool) IoBuffer {
//	if head {
//		if err := proto.Unmarshal(nb.ReadDataWithHead(), v); err != nil {
//			panic(err)
//		}
//	} else {
//		if err := proto.Unmarshal(nb.ReadData(), v); err != nil {
//			panic(err)
//		}
//	}
//	return nb
//}

var mh codec.MsgpackHandle

//将 v 通过 "github.com/ugorji/go/codec" 编码写入到buffer中 head:是否有消息体长度
var WriteMsgpData = func(nb IoBuffer, v interface{}, head bool) {
	var w bytes.Buffer
	if err := codec.NewEncoder(&w, &mh).Encode(v); err == nil {
		if head {
			nb.WriteDataWithHead(w.Bytes())
		} else {
			nb.WriteData(w.Bytes())
		}
	} else {
		panic(err)
	}

}

//读取一个data通过 "github.com/ugorji/go/codec" 解码到v(a pointer interface{}) head:是否有消息体长度
var ReadMsgpData = func(nb IoBuffer, v interface{}, head bool) {
	if head {
		if err := codec.NewDecoderBytes(nb.ReadDataWithHead(), &mh).Decode(v); err != nil {
			panic(err)
		}
	} else {
		if err := codec.NewDecoderBytes(nb.ReadData(), &mh).Decode(v); err != nil {
			panic(err)
		}
	}
}

//将 v 通过 "encoding/gob" 编码写入到buffer中 head:是否有消息体长度
var WriteGobData = func(nb IoBuffer, v interface{}, head bool) {
	var w bytes.Buffer
	if err := gob.NewEncoder(&w).Encode(v); err == nil {
		if head {
			nb.WriteDataWithHead(w.Bytes())
		} else {
			nb.WriteData(w.Bytes())
		}
	} else {
		panic(err)
	}
}

//读取一个data通过 "encoding/gob" 解码到v(a pointer interface{}) head:是否有消息体长度
var ReadGobData = func(nb IoBuffer, v interface{}, head bool) {
	if head {
		if err := gob.NewDecoder(bytes.NewReader(nb.ReadDataWithHead())).Decode(v); err != nil {
			panic(err)
		}
	} else {
		if err := gob.NewDecoder(bytes.NewReader(nb.ReadData())).Decode(v); err != nil {
			panic(err)
		}
	}
}

//主动调用该方法更改interface{}默认序列化"encoding/gob"方式
func InitSerialization(serialization Serialization, deSerialization DeSerialization) {
	Default_Serialization = serialization
	Default_DeSerialization = deSerialization
}

func NewBuffer(port PackApi, buf []byte) IoBuffer {
	b := &nbuffer{port: port, buf: bytes.NewBuffer(buf), uuid: createUuid()}
	return b
}

func NewCapBuffer(port PackApi, caps int) IoBuffer {
	buf := buffpool.BufGet(caps)
	buf = buf[:0]
	b := &nbuffer{port: port, buf: bytes.NewBuffer(buf), uuid: createUuid()}
	return b
}
