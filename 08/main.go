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
	trimName := strings.Split(strings.TrimSuffix(filename, "/"), "/")
	staticName := "@" + trimName[len(trimName)-1]
	outFile := ""
	files := []string{}

	switch {
	case strings.HasSuffix(filename, ".vm"):
		outFile = strings.TrimSuffix(filename, "vm") + "asm"
		files = append(files, filename)
		staticName = strings.TrimSuffix(staticName, "vm")
	default:
		outFile = filename + "/" + trimName[len(trimName)-1] + ".asm"
		dirFiles, err := os.ReadDir(filename)
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range dirFiles {
			if strings.HasSuffix(f.Name(), ".vm") {
				files = append(files, filename+"/"+f.Name())
			}
		}
		staticName += "."
	}

	fmt.Printf("Translating %s\n", filename)
	fmt.Printf("Assembly code at %s\n", outFile)

	// output file
	ofile, err := os.OpenFile(outFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer ofile.Close()

	fmt.Printf("Static name %s\n", staticName)

	w := NewCodeWriter(ofile, staticName)
	w.bootstrap()

	for _, f := range files {
		// input file
		file, err := os.Open(f)
		if err != nil {
			fmt.Println("Error reading filename: ", err)
			return
		}
		defer file.Close()

		p := NewParser(file)
		sn := strings.Split(strings.TrimSuffix(f, "vm"), "/")
		w.staticName = "@" + sn[len(sn)-1]

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
				case cmdType == "C_LABEL":
					label := p.arg1()
					w.writeLabel(label)
				case cmdType == "C_GOTO":
					label := p.arg1()
					w.writeGoto(label)
				case cmdType == "C_IF":
					label := p.arg1()
					w.writeIf(label)
				case cmdType == "C_FUNCTION":
					fnName := p.arg1()
					nVars := p.arg2()
					w.writeFunction(fnName, nVars)
				case cmdType == "C_CALL":
					fnName := p.arg1()
					nArgs := p.arg2()
					w.writeCall(fnName, nArgs)
				case cmdType == "C_RETURN":
					w.writeReturn()
				}
			}
		}
	}
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
	case cmd == "label":
		return "C_LABEL"
	case cmd == "goto":
		return "C_GOTO"
	case cmd == "if-goto":
		return "C_IF"
	case cmd == "function":
		return "C_FUNCTION"
	case cmd == "return":
		return "C_RETURN"
	case cmd == "call":
		return "C_CALL"
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
	retCount   int
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

// write a section (LABEL) in assembly
func (c *CodeWriter) writeLabel(s string) {
	asm := "(" + s + ")\n"
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// write an unconditional jump in assembly
func (c *CodeWriter) writeGoto(s string) {
	asm := "\t@" + s + "\n\t0;JMP\n"
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// write a conditional jump in assembly, based on results in stack
func (c *CodeWriter) writeIf(s string) {
	asm := popD() + "\t@" + s + "\n\tD;JNE\n"
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// writeFunction initializes the local variables of the callee
func (c *CodeWriter) writeFunction(fnName string, nVars int) {
	asm := "(" + fnName + ")\n"
	for i := 0; i < nVars; i++ {
		asm += constD("0") + pushD() + incrementSP()
	}
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// writeCall saves the frame of the caller (on the satck) and jumps to execute the called function
func (c *CodeWriter) writeCall(fnName string, nArgs int) {
	// push returnAddr
	asm := "\t@" + fnName + "$ret" + strconv.Itoa(c.retCount) + "\n\tD=A\n" + pushD() + incrementSP()

	// push LCL, ARG, THIS, THAT
	asm += "\t@LCL\n\tD=M\n" + pushD() + incrementSP() + "\t@ARG\n\tD=M\n" + pushD() + incrementSP() +
		"\t@THIS\n\tD=M\n" + pushD() + incrementSP() + "\t@THAT\n\tD=M\n" + pushD() + incrementSP()

	// ARG = SP-5-nArgs
	asm += "\t@" + strconv.Itoa(5+nArgs) + "\n\tD=A\n\t@SP\n\tD=M-D\n\t@ARG\n\tM=D\n"
	// LCL=SP
	asm += "\t@SP\n\tD=M\n\t@LCL\n\tM=D\n"

	// goto function
	asm += "\t@" + fnName + "\n\t0;JMP\n"
	// ( returnAddr )
	asm += "(" + fnName + "$ret" + strconv.Itoa(c.retCount) + ")\n"

	c.retCount++
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// writeReturn copies the return value to the top of the caller's working stack, reinstates the segment pointers of the caller, and jumps to the returnAddress in the caller
func (c *CodeWriter) writeReturn() {
	// frame = LCL
	asm := "\t@LCL\n\tD=A\n\tD=M\n\t@frame\n\tM=D\n"
	// retAddr = *(frame-5)
	asm += constD("5") + "\t@frame\n\tD=M-D\n\tA=D\n\tD=M\n\t@retAddr\n\tM=D\n"
	// *ARG = pop()
	asm += popD() + "\t@ARG\n\tA=M\n\tM=D\n"
	// SP = ARG + 1
	asm += "\t@ARG\n\tD=M+1\n\t@SP\n\tM=D\n"

	// THAT = *(frame-1)
	asm += constD("1") + "\t@frame\n\tD=M-D\n\tA=D\n\tD=M\n\t@THAT\n\tM=D\n"
	// THIS = *(frame-2)
	asm += constD("2") + "\t@frame\n\tD=M-D\n\tA=D\n\tD=M\n\t@THIS\n\tM=D\n"
	// ARG = *(frame-3)
	asm += constD("3") + "\t@frame\n\tD=M-D\n\tA=D\n\tD=M\n\t@ARG\n\tM=D\n"
	// LCL = *(frame-4)
	asm += constD("4") + "\t@frame\n\tD=M-D\n\tA=D\n\tD=M\n\t@LCL\n\tM=D\n"
	// goto retAddr
	asm += "\t@retAddr\n\tA=M\n\t0;JMP\n"
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
}

// bootstrap the file
func (c *CodeWriter) bootstrap() {
	asm := "// initialize program state\n(bootstrap)\n" + constD("256") + "\t@SP\n\tM=D\n" // +
	if _, err := c.stream.WriteString(asm); err != nil {
		log.Println(err)
	}
	c.writeCall("Sys.init", 0)
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
	phrase := popD() + decrementSP() + "\tA=M\n"
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
