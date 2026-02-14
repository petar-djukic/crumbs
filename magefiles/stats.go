// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
)

// Stats prints Go lines of code and documentation word counts.
func Stats() error {
	rec, err := newOrch().CollectStats()
	if err != nil {
		return err
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	fmt.Println(string(line))
	return nil
}
