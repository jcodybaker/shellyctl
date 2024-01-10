package gencobra

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
)

func RequestToCmd(req shelly.RPCRequestBody) (*cobra.Command, error) {
	c := &cobra.Command{}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if err := forEachStructField(req, "", newFlagReader(c.Flags(), req.Method())); err != nil {
			return err
		}
		return nil
	}
	var err error
	if c.Use, err = transformMethodName(req.Method()); err != nil {
		return nil, err
	}
	if err := forEachStructField(req, "", newFlagFactory(c.Flags(), req.Method())); err != nil {
		return c, err
	}

	return c, nil
}

func transformMethodName(n string) (string, error) {
	_, subCommand, ok := strings.Cut(n, ".")
	if !ok {
		return "", errors.New("failed to parse method name")
	}
	return strcase.KebabCase(subCommand), nil
}

// func requestToFlags(req shelly.RPCRequestBody, f *pflag.FlagSet) error {
// 	v := reflect.ValueOf(req)
// 	if k := v.Kind(); k != reflect.Pointer {
// 		return fmt.Errorf("req shelly.RPCRequestBody interface had invalid type / value: %T", req)
// 	}
// 	v = v.Elem()
// 	if k := v.Kind(); k != reflect.Struct {
// 		return fmt.Errorf("req *shelly.RPCRequestBody interface had invalid type / value: %T", req)
// 	}
// 	t := v.Type()
// 	for i := 0; i < t.NumField(); i++ {
// 		ft := t.Field(i)
// 		if !ft.IsExported() {
// 			continue
// 		}
// 		jTag, ok := ft.Tag.Lookup("json")
// 		if !ok {
// 			continue
// 		}
// 		name, _, _ := strings.Cut(jTag, ",")
// 		if err := typeToFlag(f, ft.Type, name, "", req.Method()); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

type something func(fieldType reflect.Type, name, prefix string) error

func forEachStructField(s interface{}, prefix string, f something) error {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if k := v.Kind(); k != reflect.Struct {
		return fmt.Errorf("req *shelly.RPCRequestBody interface had invalid type / value: %T", s)
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		jTag, ok := field.Tag.Lookup("json")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(jTag, ",")
		fieldType := field.Type
		fv := v.Field(i)
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
			if fv.IsNil() {
				fv = reflect.New(fieldType)
			} else {
				fv = fv.Elem()
			}
		}

		if fieldType.Kind() == reflect.Struct {
			var newPrefix string
			if prefix != "" || name != "config" {
				if prefix != "" {
					newPrefix = prefix + "-"
				}
				newPrefix = newPrefix + strcase.KebabCase(name)
			}
			if err := forEachStructField(fv.Interface(), newPrefix, f); err != nil {
				return err
			}
		} else {
			if err := f(fieldType, name, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func newFlagReader(f *pflag.FlagSet, method string) something {
	ff := func(typ reflect.Type, name, prefix string) error {
		flagName := strcase.KebabCase(name)
		if prefix != "" {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		fv := f.Lookup(flagName)
		fmt.Printf("--%s=%v set=%v\n", flagName, fv.Changed, fv.Value)
		return nil
	}
	return ff
}

func newFlagFactory(f *pflag.FlagSet, method string) something {
	var ff something
	ff = func(typ reflect.Type, name, prefix string) error {
		flagName := strcase.KebabCase(name)
		if prefix != "" {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		desc := fmt.Sprintf("Set the %q field on the %q request", name, method)
		if gotFlag := f.Lookup(flagName); gotFlag != nil {
			return nil
		}
		switch typ.Kind() {
		case reflect.Pointer:
			return ff(typ.Elem(), name, prefix)
		case reflect.String:
			f.String(flagName, "", desc)
		case reflect.Bool:
			desc := fmt.Sprintf(
				"Set the %q field on the %q request; set --%s=false to disable.",
				name,
				method,
				flagName)
			f.Bool(flagName, false, desc)
		case reflect.Int:
			f.Int(flagName, 0, desc)
		case reflect.Int8:
			f.Int8(flagName, 0, desc)
		case reflect.Int16:
			f.Int16(flagName, 0, desc)
		case reflect.Int32:
			f.Int32(flagName, 0, desc)
		case reflect.Int64:
			f.Int64(flagName, 0, desc)
		case reflect.Uint:
			f.Uint(flagName, 0, desc)
		case reflect.Uint8:
			f.Uint8(flagName, 0, desc)
		case reflect.Uint16:
			f.Uint16(flagName, 0, desc)
		case reflect.Uint32:
			f.Uint32(flagName, 0, desc)
		case reflect.Uint64:
			f.Uint64(flagName, 0, desc)
		case reflect.Float32:
			f.Float32(flagName, 0, desc)
		case reflect.Float64:
			f.Float64(flagName, 0, desc)
		default:
			return fmt.Errorf("unknown type: %v", typ.Kind())
		}
		return nil
	}
	return ff
}
