package main

import (
	"flag"
	"fmt"
	pwe "github.com/kuking/go-pwentropy"
	"github.com/kuking/seof"
	"io"
	"io/ioutil"
	"os"
)

var doDecrypt bool
var passwordFile string
var doHelp bool
var doInfo bool
var blockSize uint

func doArgsParsing() bool {
	flag.BoolVar(&doDecrypt, "d", false, "decrypt (default: to encrypt)")
	flag.BoolVar(&doInfo, "i", false, "show seof encrypted file metadata")
	flag.StringVar(&passwordFile, "p", "", "password file")
	flag.UintVar(&blockSize, "s", 1024, "block size (default: 1024)")
	flag.BoolVar(&doHelp, "h", false, "Show usage")
	flag.Parse()
	if doHelp || flag.NArg() != 1 {
		fmt.Printf("Usage of %v: seof file utility\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Print(`
NOTES: 
  - Password must to be provided in a file. Command line is not secure in a multi-user host.
  - When encrypting, contents have to be provided via a pipe file, while decrypting output is always to stdout.

Examples: 
  $ cat file | seof -p @password_file file.seof
  $ seof -d -p @password_file file.seof > file
  $ seof -i -p @password_file file.seof 
`)
		return false
	}
	return true
}

func main() {

	if !doArgsParsing() {
		os.Exit(-1)
	}

	if passwordFile == "" {
		_, _ = os.Stderr.WriteString("password not provided.\n")
		os.Exit(-1)
	}

	if len(passwordFile) > 1 && passwordFile[0] == '@' {
		passwordFile = passwordFile[1:]
	}
	passwordBytes, err := ioutil.ReadFile(passwordFile)
	if err != nil {
		panic(err)
	}
	password := string(passwordBytes)

	entropy := pwe.FairEntropy(password)
	if entropy < 96 {
		os.Stderr.WriteString(fmt.Sprintf("FATAL: Est. entropy for provided password is not enough: %2.2f (minimum: 96)\n\n", entropy))
		password = pwe.PwGen(pwe.FormatEasy, pwe.Strength256)
		entropy = pwe.FairEntropy(password)
		os.Stderr.WriteString(fmt.Sprintf("We have created a password for you with %2.2f bits of entropy \n"+
			"+-------------------------------------------------------+\n"+
			"| %52v  |\n"+
			"+-------------------------------------------------------+\n", entropy, password))
		os.Exit(-1)
	}

	filename := os.Args[len(os.Args)-1]
	var ef *seof.File
	if doInfo || doDecrypt {
		ef, err = seof.OpenExt(filename, password, 10)
	} else {
		ef, err = seof.CreateExt(filename, password, int(blockSize), 10)
	}
	assertNoError(err, "Failed to open file: "+filename+" -- %v")

	if doInfo {
		stats, err := ef.Stat()
		assertNoError(err, "FATAL: problems doing file stats %v")

		fmt.Println("         Name:", stats.Name())
		fmt.Println("    Mod. Time:", stats.ModTime())
		fmt.Printf("    File Mode: %v \n", stats.Mode())
		fmt.Printf(" Content Size: %v bytes\n", stats.Size())
		fmt.Printf("    File Size: %v bytes (disk occupied)\n", stats.EncryptedSize())
		fmt.Printf("     Overhead: %2.2f%% (encryption) \n", float32(stats.EncryptedSize()) * 100 / float32(stats.Size())-100)
		fmt.Printf("Content Block: %v bytes (before encryption)\n", stats.BEBlockSize())
		fmt.Printf("   Disk Block: %v bytes (after encryption; on disk)\n", stats.DiskBlockSize())
		fmt.Printf("Blocks Writen: %v (= unique nonces written)\n", stats.BlocksWritten())

	} else if doDecrypt {
		_, err = io.Copy(os.Stdout, ef)
	} else {
		_, err = io.Copy(ef, os.Stdin)
	}
	assertNoError(err, "FATAL: io error: %v")

	err = ef.Close()
	assertNoError(err, "FATAL: could not close the seof file: %v")
}

func assertNoError(err error, pattern string) {
	if err != nil {
		_, _ = os.Stderr.WriteString(fmt.Sprintf(pattern+"\n", err))
		os.Exit(-1)
	}
}
