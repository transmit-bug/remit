package main

import "github.com/MoeForge/moejs"

func spinAndReturn(rt *moejs.Runtime, h moejs.Hook) error {
	_, err := rt.Call(h)
	return err
}
