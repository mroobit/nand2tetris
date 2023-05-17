// This file is part of www.nand2tetris.org
// and the book "The Elements of Computing Systems"
// by Nisan and Schocken, MIT Press.
// File name: projects/04/Fill.asm

// Runs an infinite loop that listens to the keyboard input.
// When a key is pressed (any key), the program blackens the screen,
// i.e. writes "black" in every pixel;
// the screen should remain fully black as long as the key is pressed. 
// When no key is pressed, the program clears the screen, i.e. writes
// "white" in every pixel;
// the screen should remain fully clear as long as no key is pressed.

(CONT_LOOP)
	@SCREEN
	D=A
	@addr		// start of screen pixels in memory
	M=D

	@8192		// total number of pixels to fill
	D=D+A
	@pixels		// memory location of last of pixesl
	M=D

	@color
	M=0

	@KBD
	D=M
	@FILL_SCREEN	// if KBD is zero (no input), jump to fill with color=0
	D;JEQ

	@color		// set color to -1 (if no input, screen will fill)
	M=-1

(FILL_SCREEN)		// loop to fill screen
	@addr
	D=M
	@pixels
	D=D-M		// subtract current pixel from starting address
	@CONT_LOOP
	D;JEQ		// if current pixel is start, screen-fill is complete, return to main loop

	@color		// fill with color set in main loop
	D=M
	@addr
	A=M
	M=D

	@addr		// advance to next pixel to fill
	M=M+1
	@FILL_SCREEN
	0;JMP		// jump to top of fill loop
