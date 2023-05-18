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

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error reading filename: ", err)
		return
	}
	defer file.Close()

	outFile := strings.TrimSuffix(filename, "asm") + "hack"

	fmt.Printf("Translating %s\n", filename)
	fmt.Printf("Machine code at %s\n", outFile)

	tf, err := os.OpenFile(outFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer tf.Close()

	acTable := map[string]string{
		"0":   "0101010",
		"1":   "0111111",
		"-1":  "0111010",
		"D":   "0001100",
		"A":   "0110000",
		"M":   "1110000",
		"!D":  "0001101",
		"!A":  "0110001",
		"!M":  "1110001",
		"-D":  "0001111",
		"-A":  "0110011",
		"-M":  "1110011",
		"D+1": "0011111",
		"A+1": "0110111",
		"M+1": "1110111",
		"D-1": "0001110",
		"A-1": "0110010",
		"M-1": "1110010",
		"D+A": "0000010",
		"D+M": "1000010",
		"D-A": "0010011",
		"D-M": "1010011",
		"A-D": "0000111",
		"M-D": "1000111",
		"D&A": "0000000",
		"D&M": "1000000",
		"D|A": "0010101",
		"D|M": "1010101",
	}

	dTable := map[string]string{
		"":    "000",
		"M":   "001",
		"D":   "010",
		"MD":  "011",
		"A":   "100",
		"AM":  "101",
		"AD":  "110",
		"ADM": "111",
	}

	jTable := map[string]string{
		"":    "000",
		"JGT": "001",
		"JEQ": "010",
		"JGE": "011",
		"JLT": "100",
		"JNE": "101",
		"JLE": "110",
		"JMP": "111",
	}

	numLines := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case len(line) > 0 && string(line[0]) == "@":
			numLines++
			v, _ := strconv.Atoi(line[1:])
			binRep := strconv.FormatInt(int64(v), 2)
			extra := 16 - len(binRep)
			for i := 0; i < extra; i++ {
				binRep = "0" + binRep
			}
			binRep = binRep + "\n"
			if _, err := tf.WriteString(binRep); err != nil {
				log.Println(err)
			}
		case len(line) > 0 && string(line[0:2]) != "//":
			numLines++
			// translate all of the bits based on line format
			// dest = comp; jump
			// split by "=", then by ";"
			// 111 a cccccc ddd jjj
			instr := "111"
			// Could lack =, Could lack ;

			cbits := strings.FieldsFunc(line, Delim)

			dest := ""
			comp := ""
			jump := ""
			switch {
			case len(cbits) == 3:
				dest = cbits[0]
				comp = cbits[1]
				jump = cbits[2]
			case len(cbits) == 2 && strings.Contains(line, "="):
				dest = cbits[0]
				comp = cbits[1]
			case len(cbits) == 2 && strings.Contains(line, ";"):
				comp = cbits[0]
				jump = cbits[1]
			case len(cbits) == 1:
				comp = cbits[0]
			}

			instr = instr + acTable[comp] + dTable[dest] + jTable[jump] + "\n"

			if _, err := tf.WriteString(instr); err != nil {
				log.Println(err)
			}
		}

	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

}

func Delim(r rune) bool {
	return string(r) == "=" || string(r) == ";"
}
