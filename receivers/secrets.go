package receivers

import (
	"context"
	"reflect"
	"strings"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

type decryptFunc = func(key string, fallback string) string

// Secret is type that should be used for fields that contain any sensitive information like passwords and tokens.
// Once notifier configuration fields are marked by that field, this field value will be taken from a secret settings during the unmarshalling.
// Applicable only if extension SecretsDecoderExt is registered in the marshaller instance.
type Secret string

// secretsDecoderDecorator decorates a struct decoder to
type secretsDecoderDecorator struct {
	original      jsoniter.ValDecoder
	decryptFunc   decryptFunc
	decryptFields map[string]reflect2.StructField
}

func (s secretsDecoderDecorator) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	s.original.Decode(ptr, iter)
	if iter.Error != nil {
		return
	}
	for key, field := range s.decryptFields {
		fieldVal := field.UnsafeGet(ptr)
		originalValue := string(*((*Secret)(fieldVal)))
		decrypted := s.decryptFunc(key, originalValue)
		if decrypted != originalValue {
			*((*Secret)(fieldVal)) = Secret(decrypted)
		}
	}
}

type secretsDecoderExtension struct {
	jsoniter.DummyExtension
	decrypt decryptFunc
}

func (s *secretsDecoderExtension) DecorateDecoder(typ reflect2.Type, decoder jsoniter.ValDecoder) jsoniter.ValDecoder {
	if typ.Kind() != reflect.Struct {
		return decoder
	}
	decryptFields := make(map[string]reflect2.StructField, 4)
	structType := typ.(*reflect2.UnsafeStructType)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Type() != reflect2.TypeOf(Secret("")) {
			continue
		}
		tag := strings.Split(field.Tag().Get("json"), ",")[0]
		if tag == "" {
			tag = field.Name()
		}
		decryptFields[tag] = field
	}
	if len(decryptFields) == 0 {
		return decoder
	}
	return &secretsDecoderDecorator{
		original:      decoder,
		decryptFunc:   s.decrypt,
		decryptFields: decryptFields,
	}
}

func CreateMarshallerWithSecretsDecrypt(decryptFunc GetDecryptedValueFn, secrets map[string][]byte) jsoniter.API {
	// not all receivers do need secure settings, we still might interact with
	// them, so we make sure they are never nil
	if secrets == nil {
		secrets = map[string][]byte{}
	}
	var j = jsoniter.Config{ // this is jsoniter.ConfigCompatibleWithStandardLibrary
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
	}.Froze()
	// we have to create API every time because we have secrets enclosed in the config.
	ext := &secretsDecoderExtension{
		decrypt: func(key string, fallback string) string {
			return decryptFunc(context.Background(), secrets, key, fallback)
		},
	}
	j.RegisterExtension(ext)
	return j
}
