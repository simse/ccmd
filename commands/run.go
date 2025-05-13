package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/simse/cmd-cache/internal"
)

// formatting utils
var dimGrey = color.RGB(100, 100, 100)

type RunCommandArgs struct {
	Input            []string `arg:"-i,--input,required"`
	Output           []string `arg:"required"`
	Command          string   `arg:"required"`
	WorkingDirectory string   `arg:"--cwd"`
	Profile          bool     `arg:"--profile"`
}

func RunCommand(args *RunCommandArgs) {
	var profiling struct {
		FindFiles        time.Duration
		HashFiles        time.Duration
		CacheLookup      time.Duration
		CacheExtract     time.Duration
		CommandExecution time.Duration
		CacheSave        time.Duration
	}

	// determine working directory
	workingDirectory := args.WorkingDirectory

	if workingDirectory == "" {
		workingDirectory, _ = os.Getwd()
	}

	absoluteWorkingDirectory, _ := filepath.Abs(workingDirectory)

	// validate args
	for _, inputPattern := range args.Input {
		if strings.Contains(inputPattern, "../") {
			fmt.Println("! input pattern cannot be relative, use --cwd to change to parent directory")
			os.Exit(1)
		}
	}

	fmt.Print("Using working directory: ")
	color.Cyan(absoluteWorkingDirectory)

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

	dimGrey.Printf("Found %d input files in %s\n", len(inputFiles), formatDuration(profiling.FindFiles))

	// calculate hash of inputs
	hashFilesStart := time.Now()
	inputChecksum, err := internal.HashDir(inputFiles, workingDirectory)
	profiling.HashFiles = time.Since(hashFilesStart)

	if err != nil {
		fmt.Println("error while hashing input files")
		os.Exit(1)
	}

	dimGrey.Printf("Computed cache key %s in %s\n", inputChecksum, formatDuration(profiling.HashFiles))

	// check cache
	cacheLookupStart := time.Now()
	existsInCache := internal.CacheKeyExists(inputChecksum)
	profiling.CacheLookup = time.Since(cacheLookupStart)

	// if cache exists, then extract
	if existsInCache {
		// extract cache
		cacheExtractStart := time.Now()
		outputFiles, err := internal.ExtractArchive(inputChecksum, workingDirectory)
		profiling.CacheExtract = time.Since(cacheExtractStart)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		dimGrey.Printf("Cache hit: served from cache in %s\n", formatDuration(profiling.CacheLookup+profiling.CacheExtract))
		printFileList(outputFiles, 10, "->")
	} else { // otherwise execute command, then save
		dimGrey.Printf("Cache miss: executing command...\n\n")

		// run command
		fmt.Print("Running command: ")
		color.Cyan(args.Command)
		commandExecutionStart := time.Now()
		err = internal.RunCommand(args.Command, workingDirectory)

		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		profiling.CommandExecution = time.Since(commandExecutionStart)

		dimGrey.Printf("Command completed in %s\n", formatDuration(profiling.CommandExecution))

		// capture output
		saveToCacheStart := time.Now()
		outputFiles, err := internal.FindFiles(args.Output, workingDirectory)

		// fmt.Println(outputFiles)
		// return

		if err != nil {
			fmt.Println("error occured while searching for output files")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if len(outputFiles) == 0 {
			fmt.Println("did not find any matching output files, nothing to save!")
			os.Exit(1)
		}

		archiveSize, saveOutputErr := internal.CaptureOutput(outputFiles, inputChecksum, workingDirectory)
		if saveOutputErr != nil {
			fmt.Println("error occurred while saving output")
			fmt.Println(saveOutputErr.Error())
		}

		profiling.CacheSave = time.Since(saveToCacheStart)

		dimGrey.Printf("Stored result (%s) in cache in %s\n", internal.ByteCountSI(archiveSize), formatDuration(profiling.CacheSave))
		printFileList(outputFiles, 10, "+")
	}
}

func printFileList(files []string, maxFiles int, prefix string) {
	grey := color.RGB(170, 170, 170).PrintfFunc()

	for i, f := range files {
		if i == maxFiles {
			grey(" %s … and %d more files\n", prefix, len(files)-maxFiles)
			break
		}
		color.Set(color.FgGreen)
		fmt.Printf(" %s ", prefix)
		color.Set(color.FgHiWhite)
		fmt.Println(f)
	}
}

func formatDuration(dur time.Duration) string {
	ms := float64(dur.Microseconds()) / 1000
	return fmt.Sprintf("%.2fms", ms)
}
