package serial

import (
	"errors"
	"reflect"

	"github.com/golang/protobuf/proto"
)

// ToTypedMessage converts a proto Message into TypedMessage.
func ToTypedMessage(message proto.Message) *TypedMessage {
	if message == nil {
		return nil
	}
	settings, _ := proto.Marshal(message) // 编码成byte
	
	return &TypedMessage{
		Type:  GetMessageType(message),
		Value: settings,
	}
}

// GetMessageType returns the name of this proto Message.
func GetMessageType(message proto.Message) string {
	return proto.MessageName(message)
}

// GetInstance creates a new instance of the message with messageType.
func GetInstance(messageType string) (interface{}, error) {
	mType := proto.MessageType(messageType)
	if mType == nil || mType.Elem() == nil {
		return nil, errors.New("Serial: Unknown type: " + messageType)
	}
	// .Elem(): 通过指针取类型
	return reflect.New(mType.Elem()).Interface(), nil
}

// GetInstance converts current TypedMessage into a proto Message.
func (v *TypedMessage) GetInstance() (proto.Message, error) {
	/**
		type:"v2ray.core.app.log.Config" 
		value:"\x08\x01\x10\x02"
	*/
	// instance: reflect.New(mType.Elem()).Interface()
	instance, err := GetInstance(v.Type)
	if err != nil {
		return nil, err
	}
	protoMessage := instance.(proto.Message) // instance转换成proto.Message, 失败则panic

	// 解码到基类
	if err := proto.Unmarshal(v.Value, protoMessage); err != nil {
		return nil, err
	}
	return protoMessage, nil
}
