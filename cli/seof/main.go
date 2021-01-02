package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	pwe "github.com/kuking/go-pwentropy"
	"github.com/kuking/seof"
	"io"
	"io/ioutil"
	"os"
)

var doEncrypt bool
var passwordFile string
var doHelp bool
var doInfo bool
var blockSize uint

func doArgsParsing() bool {
	flag.BoolVar(&doEncrypt, "e", false, "encrypt (default: to decrypt)")
	flag.BoolVar(&doInfo, "i", false, "show seof encrypted file metadata")
	flag.StringVar(&passwordFile, "p", "", "password file")
	flag.UintVar(&blockSize, "s", 1024, "block size")
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
  $ cat file | seof -e -p @password_file file.seof
  $ seof -p @password_file file.seof > file
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
	if doInfo || !doEncrypt {
		ef, err = seof.OpenExt(filename, password, 10)
	} else {
		ef, err = seof.CreateExt(filename, password, int(blockSize), 10)
	}
	assertNoError(err, "Failed to open file: "+filename+" -- %v")

	if doInfo {
		stats, err := ef.Stat()
		assertNoError(err, "FATAL: problems doing file stats %v")

		fmt.Printf("           File Name: %v\n", stats.Name())
		fmt.Printf("   Modification Time: %v\n", stats.ModTime())
		fmt.Printf("           File Mode: %v \n", stats.Mode())
		fmt.Printf("        Content Size: %v bytes\n", stats.Size())
		fmt.Printf("   File Size On Disk: %v bytes\n", stats.EncryptedSize())
		fmt.Printf(" Encryption Overhead: %2.2f%%\n", float32(stats.EncryptedSize())*100/float32(stats.Size())-100)
		fmt.Printf("  Content Block Size: %v bytes\n", stats.BEBlockSize())
		fmt.Printf("Encrypted Block Size: %v bytes\n", stats.DiskBlockSize())
		fmt.Printf(" Total Blocks Writen: %v (= unique nonces)\n", stats.BlocksWritten())
		salt, n, r, p := stats.SCryptParameters()
		fmt.Printf("   SCrypt Parameters: N=%v, R=%v, P=%v, keyLength=96, salt=\n", n, r, p)
		hexa := hex.EncodeToString(salt)
		fmt.Printf("%69v\n%69v\n%69v\n", hexa[:64], hexa[64:128], hexa[128:])

	} else if doEncrypt {
		_, err = io.Copy(ef, os.Stdin)
	} else {
		_, err = io.Copy(os.Stdout, ef)
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
