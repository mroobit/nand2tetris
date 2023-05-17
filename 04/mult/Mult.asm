// This file is part of www.nand2tetris.org
// and the book "The Elements of Computing Systems"
// by Nisan and Schocken, MIT Press.
// File name: projects/04/Mult.asm

// Multiplies R0 and R1 and stores the result in R2.
// (R0, R1, R2 refer to RAM[0], RAM[1], and RAM[2], respectively.)
//
// This program only needs to handle arguments that satisfy
// R0 >= 0, R1 >= 0, and R0*R1 < 32768.

// asssess which is smaller value
// loop for smaller value times
// adding greater value to memory slot 2 each time
// then end

	@R2
	M=0

	@R0
	D=M
	@R1
	D=D-M
	@R0_BIGGER	// if R0 > R1, jump to R0_BIGGER
	D;JGT

	// if R1 is greater, set incrementing value to RAM[1]
	@R1
	D=M
	@incr
	M=D

	@R0
	D=M
	@loopct	// set loop counter to smaller value
	M=D
	@LOOP
	0;JMP

(R0_BIGGER)
	@R0
	D=M
	@incr	// set the incrementing value to RAM[0]
	M=D

	@R1
	D=M
	@loopct
	M=D

(LOOP)
	@loopct
	D=M
	@END
	D;JEQ	// if loopct has been decremented to 0, jump to END

	@R2	// load running sum
	D=M
	@incr
	D=D+M	// add incrementing value to running sum
	@R2
	M=D	// write new sum to @sum

	@loopct
	D=M-1	// decrement loopct
	M=D	// write new val to loopct
	@LOOP
	0;JMP	// jump to top of loop

(END)
	@END
	0;JMP

