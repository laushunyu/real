package main

import (
	"fmt"
	"reflect"
)

type EventChunkChange struct {
	player *Player

	SrcX, SrcZ, DstX, DstZ int64
}

type EventCenter struct {
	handlers map[reflect.Type][]reflect.Value
}

// Send event to its handlers
func (ec *EventCenter) Send(evt any) {
	evtV := reflect.Indirect(reflect.ValueOf(evt))

	handlers, ok := ec.handlers[evtV.Type()]
	if !ok {
		return
	}

	for _, handler := range handlers {
		handler.Call([]reflect.Value{evtV})
	}
}

// On register a handler when event(get from reflect.In(0)) happened.
// handler must like `func(evt Event)`.
// all handler should be registered after start to avoid race.
func (ec *EventCenter) On(handler any) {
	value := reflect.ValueOf(handler)
	typ := value.Type()

	// valid
	if typ.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	if typ.NumIn() != 1 || typ.NumOut() != 0 {
		panic("handler must like `func(evt Event)`")
	}

	// get event
	evt := typ.In(0)
	for evt.Kind() == reflect.Ptr {
		evt = evt.Elem()
	}
	ec.handlers[evt] = append(ec.handlers[evt], value)
}

var ec = &EventCenter{
	handlers: make(map[reflect.Type][]reflect.Value),
}

func init() {
	ec.On(func(evt EventChunkChange) {
		evt.player.SendChat(Chat{
			Text: fmt.Sprintf("[%s]", evt.player.Meta.User),
			Bold: true,
			Extra: []Chat{
				{
					Text: " Move to from chunk",
					Bold: false,
				},
				{
					Text: fmt.Sprintf(" (%d, %d) ", evt.SrcX, evt.SrcZ),
					Bold: true,
				},
				{
					Text: "to chunk",
					Bold: false,
				},
				{
					Text: fmt.Sprintf("(%d, %d)", evt.DstX, evt.DstZ),
					Bold: true,
				},
			},
		})
	})
}
