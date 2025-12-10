package form

import (
	"reflect"
	"strconv"

	"github.com/google/uuid"
	"github.com/manifoldco/promptui"
)

type Form[T any] struct {
	err error
}

type Runner interface {
	Run() (string, error)
}

type Selector interface {
	Run() (int, string, error)
}

func (f *Form[T]) Parse() (T, error) {
	var t T
	e := reflect.ValueOf(&t).Elem()
	for _, v := range reflect.VisibleFields(e.Type()) {
		if !v.IsExported() {
			continue
		}
		name := v.Name
		field := e.FieldByName(name)
		if name == "Uuid" {
			field.SetString(uuid.NewString())
			continue
		}
		if name == "CreatedAt" {
			continue
		}
		prompt := &promptui.Prompt{
			Label: name,
		}
		var v string
		var err error
		switch field.Type().Name() {
		case "int",
			"string":
			v, err = prompt.Run()
		default:
			continue
		}
		if err != nil {
			panic(err)
		}
		switch field.Type().Name() {
		case "int":
			i, err := strconv.Atoi(v)
			if err != nil {
				panic(err)
			}
			field.SetInt(int64(i))
		case "string":
			field.SetString(v)
		}
	}
	return t, nil
}

func (f *Form[T]) Add(field interface{}, fun Runner) {
	if f.err != nil {
		return
	}
	val, err := fun.Run()
	if err != nil {
		f.err = err
		return
	}
	switch f := field.(type) {
	case *bool:
		*f = len(val) > 0 && (val[0] == 't' || val[0] == 'T')
	case *string:
		*f = val
	}
}

func (f *Form[T]) AddSelect(field interface{}, fun Selector) {
	if f.err != nil {
		return
	}
	_, val, err := fun.Run()
	if err != nil {
		f.err = err
		return
	}
	switch f := field.(type) {
	case *bool:
		*f = len(val) > 0 && (val[0] == 't' || val[0] == 'T')
	case *string:
		*f = val
	}
}

func (f *Form[T]) Valid() error {
	return f.err
}
