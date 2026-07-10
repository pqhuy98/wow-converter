package ansi

import "fmt"

const reset = "\x1b[0m"

func Gray(s string) string   { return "\x1b[90m" + s + reset }
func Red(s string) string    { return "\x1b[31m" + s + reset }
func Green(s string) string  { return "\x1b[32m" + s + reset }
func Yellow(s string) string { return "\x1b[33m" + s + reset }
func Blue(s string) string   { return "\x1b[34m" + s + reset }
func Cyan(s string) string   { return "\x1b[36m" + s + reset }

func Grayf(format string, args ...any) string   { return Gray(fmt.Sprintf(format, args...)) }
func Redf(format string, args ...any) string    { return Red(fmt.Sprintf(format, args...)) }
func Greenf(format string, args ...any) string  { return Green(fmt.Sprintf(format, args...)) }
func Yellowf(format string, args ...any) string { return Yellow(fmt.Sprintf(format, args...)) }
func Bluef(format string, args ...any) string   { return Blue(fmt.Sprintf(format, args...)) }
func Cyanf(format string, args ...any) string   { return Cyan(fmt.Sprintf(format, args...)) }
