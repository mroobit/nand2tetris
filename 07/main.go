package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	filename := os.Args[1]

	// input file
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error reading filename: ", err)
		return
	}
	defer file.Close()

	// output file
	outFile := strings.TrimSuffix(filename, "vm") + "asm"
	ofile, err := os.OpenFile(outFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer ofile.Close()

	fmt.Printf("Translating %s\n", filename)
	fmt.Printf("Assembly code at %s\n", outFile)

	// static name
	trimName := strings.Split(filename, "/")
	staticName := "@" + strings.TrimSuffix(trimName[len(trimName)-1], "vm")

	p := NewParser(file)
	w := NewCodeWriter(ofile, staticName)

	pass := bufio.NewScanner(p.stream)
	for pass.Scan() {
		line := pass.Text()
		if len(line) > 0 && line != "\n" && line[0:2] != "//" {
			p.current = line
			w.writeComment(p.current)
			cmdType := p.commandType()
			switch {
			case cmdType == "C_ARITHMETIC":
				command := p.arg1()
				w.writeArithmetic(command)
			case cmdType == "C_PUSH" || cmdType == "C_POP":
				segment := p.arg1()
				index := p.arg2()
				if err != nil {
					log.Println(err)
				}
				w.writePushPop(cmdType, segment, index)
			}
		}
	}
	w.writeInfiniteLoop()
}

// Parser holds the input stream for parsing and current command
type Parser struct {
	stream  *os.File
	current string
}

// NewParser creates a new Parser
func NewParser(s *os.File) *Parser {
	p := &Parser{
		stream: s,
	}
	return p
}

// returns a constant representing the type of the current comand. If current command is arithmetic-logical command, returns C_ARITHMETIC
func (p *Parser) commandType() string {
	commands := strings.Fields(p.current)
	cmd := commands[0]

	switch {
	case cmd == "add" || cmd == "sub" || cmd == "neg" || cmd == "eq" || cmd == "gt" || cmd == "lt" || cmd == "and" || cmd == "or" || cmd == "not":
		return "C_ARITHMETIC"
	case cmd == "push":
		return "C_PUSH"
	case cmd == "pop":
		return "C_POP"
	}

	return "C_ARITHMETIC"
}

// returns first argument of the current command. In case of C_ARITHMETIC, command itself is returned (eg, add, sub, etc). Should not be called if current command is C_RETURN.
func (p *Parser) arg1() string {
	args := strings.Fields(p.current)
	if p.commandType() == "C_ARITHMETIC" {
		return args[0]
	}
	return args[1]
}

// returns second argument of the current command. Should only be called if current command is C_PUSH, C_POP, C_FUNCTION, or C_CALL.
func (p *Parser) arg2() int {
	args := strings.Fields(p.current)
	a2, err := strconv.Atoi(args[2])
	if err != nil {
		log.Println(err)
	}
	return a2
}

// CodeWriter holds the generated code output stream
type CodeWriter struct {
	stream     *os.File
	staticName string
	jumpCount  int
}

// NewCodeWriter creates a new CodeWriter
func NewCodeWriter(s *os.File, n string) *CodeWriter {
	c := &CodeWriter{
		stream:     s,
		staticName: n,
	}
	return c
}

// takes command and writes to the output file the assembly code that implements given arithmetic-logical command
func (c *CodeWriter) writeArithmetic(s string) {
	asm := ""
	switch {
	case s == "add" || s == "sub" || s == "or" || s == "and":
		asm += pop2(s)
	case s == "neg":
		asm += constD("0") +
			decrementSP() +
			"\tA=M\n" +
			"\tD=D-M\n"
	case s == "eq" || s == "gt" || s == "lt":
		asm += pop2("sub")
		asm += jump(s, c.jumpCount)
		c.jumpCount++
	case s == "not":
		asm += popD() + "\tD=!D\n"
	}
	asm += pushD() + incrementSP()
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// write to the output file the assembly code that implemetns the given push or pop command
func (c *CodeWriter) writePushPop(command string, segment string, index int) {
	asm := ""
	indexString := strconv.Itoa(index)

	short := map[string]string{
		"argument": "ARG",
		"local":    "LCL",
		"this":     "THIS",
		"that":     "THAT",
	}

	// push puts value of segment[index] onto stack
	if command == "C_PUSH" {
		switch {
		case segment == "argument" || segment == "local" || segment == "this" || segment == "that":
			asm += constD(indexString) + "\t@" + short[segment] + "\n\tA=M+D\n\tD=M\n"

		case segment == "constant":
			asm += constD(indexString)

		case segment == "pointer" && indexString == "0":
			asm += "\t@THIS\n\tD=M\n"
		case segment == "pointer" && indexString == "1":
			asm += "\t@THAT\n\tD=M\n"

		case segment == "static":
			asm += staticD(c.staticName + indexString)
		case segment == "temp":
			asm += "\t@" + strconv.Itoa(index+5) + "\n\tD=M\n"
		}
		asm += pushD()
		asm += incrementSP()
	}
	// pop takes top stack value and stores it in segment[index]
	if command == "C_POP" {
		switch {
		case segment == "argument" || segment == "local" || segment == "this" || segment == "that":
			asm += constD(indexString) + "\t@" + short[segment] + "\n\tD=M+D\n\t@R13\n\tM=D\n" +
				popD() + "\t@R13\n\tA=M\n\tM=D\n"

		case segment == "pointer" && indexString == "0":
			asm += popD() + "\t@THIS\n\tM=D\n"
		case segment == "pointer" && indexString == "1":
			asm += popD() + "\t@THAT\n\tM=D\n"

		case segment == "static":
			asm += popD() + fmt.Sprintf("\t%s%s\n", c.staticName, indexString) + "\tM=D\n"
		case segment == "temp":
			asm += popD() + "\t@" + strconv.Itoa(index+5) + "\n\tM=D\n"
		}
	}
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// write infinite loop at the end of the asm file
func (c *CodeWriter) writeInfiniteLoop() {
	asm := "// end of program\n(INFINITE_LOOP)\n\t@INFINITE_LOOP\n\t0;JMP"
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// writes vm line as comment
func (c *CodeWriter) writeComment(s string) {
	s = "// " + s + "\n"
	if _, err := c.stream.WriteString(s); err != nil {
		log.Println(err)
	}
}

// increment stack pointer
func incrementSP() string {
	return "\t@SP\n\tM=M+1\n"
}

// decrement stack pointer
func decrementSP() string {
	return "\t@SP\n\tM=M-1\n"
}

// pushes D to top of stack (does not include SP increment)
func pushD() string {
	return "\t@SP\n\tA=M\n\tM=D\n"
}

// pops top of stack, stores in D (includes SP decrement at start)
func popD() string {
	return "\t@SP\n\tM=M-1\n\tA=M\n\tD=M\n"
}

// stores constant in D
func constD(c string) string {
	return "\t@" + c + "\n\tD=A\n"
}

// stores static in D
func staticD(s string) string {
	return "\t" + s + "\n\tD=M\n"
}

func pop2(s string) string {
	phrase := "\t@SP\n\tM=M-1\n\tA=M\n\tD=M\n" +
		"\t@SP\n\tM=M-1\n\tA=M\n"
	switch {
	case s == "add":
		phrase += "\tD=M+D\n"
	case s == "sub":
		phrase += "\tD=M-D\n"
	case s == "and":
		phrase += "\tD=D&M\n"
	case s == "or":
		phrase += "\tD=D|M\n"
	}
	return phrase
}

func jump(j string, c int) string {
	j = "J" + strings.ToUpper(j)
	return "\t@R13\n\tM=-1\n" + // R13 true
		"\t@EVAL_" + strconv.Itoa(c) + "\n" +
		"\tD;" + j + "\n" + // JUMP, based on condition/type
		"\t@R13\n\tM=0\n" + // R13 false
		"(EVAL_" + strconv.Itoa(c) + ")\n" +
		"\t@R13\n" +
		"\tD=M\n" // D is set to true or false
}
