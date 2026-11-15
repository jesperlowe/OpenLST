//go:build !windows

package main

import "fmt"

func showError(string) {}

func showFile(path string) { fmt.Println("Resultat gemt i", path) }
