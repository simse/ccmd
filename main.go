package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/fatih/color"
	"github.com/simse/cmd-cache/internal"
)

func main() {
	var args struct {
		Input            []string `arg:"-i,--input,required"`
		Output           []string `arg:"required"`
		Command          string   `arg:"required"`
		WorkingDirectory string   `arg:"--cwd"`
		Profile          bool     `arg:"--profile"`
	}

	arg.MustParse(&args)

	var profiling struct {
		FindFiles    time.Duration
		HashFiles    time.Duration
		CacheLookup  time.Duration
		CacheExtract time.Duration
	}

	// print welcome
	printWelcome()

	// determine working directory
	workingDirectory := args.WorkingDirectory

	if workingDirectory == "" {
		workingDirectory, _ = os.Getwd()
	}

	// validate args
	for _, inputPattern := range args.Input {
		if strings.Contains(inputPattern, "../") {
			fmt.Println("! input pattern cannot be relative, use --cwd to change to parent directory")
			os.Exit(1)
		}
	}

	// find matching input files
	findFilesStart := time.Now()
	inputFiles, err := internal.FindFiles(args.Input, workingDirectory)
	profiling.FindFiles = time.Since(findFilesStart)

	if err != nil {
		fmt.Println("error occured while searching for input files")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	if len(inputFiles) == 0 {
		fmt.Println("did not find any matching files")
		os.Exit(1)
	}

	// calculate hash of inputs
	hashFilesStart := time.Now()
	inputChecksum, err := internal.HashDir(inputFiles)
	profiling.HashFiles = time.Since(hashFilesStart)

	if err != nil {
		fmt.Println("error while hashing input files")
		os.Exit(1)
	}

	fmt.Printf("found %d files, unique hash is: %s\n", len(inputFiles), inputChecksum)

	// check cache
	cacheLookupStart := time.Now()
	existsInCache := internal.CacheKeyExists(inputChecksum)
	profiling.CacheLookup = time.Since(cacheLookupStart)

	// if cache exists, then extract
	if existsInCache {
		fmt.Println("cache hit!")

		// extract cache
		outputFiles, err := internal.ExtractArchive(inputChecksum, workingDirectory)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("extracted %d files from cache\n", len(outputFiles))
	} else { // otherwise execute command, then save
		fmt.Printf("cache miss!\n")

		// pretend to execute command
		fmt.Printf("running command: %s\n", args.Command)
		err = internal.RunCommand(args.Command, workingDirectory)

		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		// capture output
		outputFiles, err := internal.FindFiles(args.Output, workingDirectory)

		if err != nil {
			fmt.Println("error occured while searching for output files")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if len(outputFiles) == 0 {
			fmt.Println("did not find any matching output files, nothing to save!")
			os.Exit(1)
		}

		archiveSize, saveOutputErr := internal.CaptureOutput(outputFiles, inputChecksum)
		if saveOutputErr != nil {
			fmt.Println("error occurred while saving output")
			fmt.Println(saveOutputErr.Error())
		}

		fmt.Printf("saved to cache with size %s\n", internal.ByteCountSI(archiveSize))
	}
	// output profiling if enabled
	if args.Profile {
		fmt.Printf("\n")
		grey := color.RGB(100, 100, 100).PrintfFunc()
		grey("findFiles: %s\n", profiling.FindFiles)
		grey("hashFiles: %s\n", profiling.HashFiles)
	}
}

func printWelcome() {
	bold := color.New(color.Bold).PrintFunc()
	grey := color.New(color.FgWhite).PrintFunc()

	bold("ccmd v0.0.1-alpha")
	grey(" - let's see if I can remember...\n")
}
