//go:build !debug

package main

import "time"

func perfStallBegin() time.Time      { return time.Time{} }
func perfStallEnd(time.Time, string) {}
func perfStallPacing(bool)           {}
