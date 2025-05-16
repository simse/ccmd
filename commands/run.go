package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/simse/cmd-cache/internal"
	"github.com/spf13/afero"
)

var AppFs = afero.NewOsFs()

// formatting utils
var dimGrey = color.RGB(100, 100, 100)

type RunCommandArgs struct {
	Input            []string `arg:"-i,--input,required"`
	Output           []string `arg:"required"`
	Command          string   `arg:"required"`
	WorkingDirectory string   `arg:"--cwd"`
	Cache            []string `arg:"--cache"`
}

var profiling struct {
	FindFiles        time.Duration
	HashFiles        time.Duration
	CacheLookup      time.Duration
	CacheExtract     time.Duration
	CommandExecution time.Duration
	CacheSaveStart   time.Time
	CacheSave        time.Duration
}

func RunCommand(args *RunCommandArgs) {

	// determine working directory
	workingDirectory := args.WorkingDirectory

	if workingDirectory == "" {
		workingDirectory, _ = os.Getwd()
	}

	absoluteWorkingDirectory, _ := filepath.Abs(workingDirectory)

	// validate args
	validateArgs(*args)

	// if no cache providers are given, fall back to local cache
	if len(args.Cache) == 0 {
		args.Cache = []string{"local://test"}
	}

	// print runtime information
	fmt.Print("Using working directory: ")
	color.Cyan(absoluteWorkingDirectory)

	fmt.Print("Cache providers to be used: ")
	color.Cyan(strings.Join(args.Cache, ", "))

	// find matching input files
	findFilesStart := time.Now()
	inputFiles, err := internal.FindFiles(args.Input, args.Output, workingDirectory)
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
	inputChecksum, err := internal.HashDir(AppFs, inputFiles, workingDirectory)
	profiling.HashFiles = time.Since(hashFilesStart)

	if err != nil {
		fmt.Println("error while hashing input files")
		os.Exit(1)
	}

	dimGrey.Printf("Computed cache key %s in %s\n", inputChecksum, formatDuration(profiling.HashFiles))

	// check cache
	cacheLookupStart := time.Now()
	cacheReader := cacheLookup(inputChecksum, args.Cache)
	profiling.CacheLookup = time.Since(cacheLookupStart)

	// if cache exists, then extract
	if cacheReader != nil {
		// extract cache
		cacheExtractStart := time.Now()
		outputFiles, err := internal.ExtractArchive(cacheReader, workingDirectory)
		profiling.CacheExtract = time.Since(cacheExtractStart)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		dimGrey.Printf("Found in cache: served %s\n", formatDuration(profiling.CacheLookup+profiling.CacheExtract))
		printFileList(outputFiles, 10, "->")
	} else { // otherwise execute command, then save
		dimGrey.Printf("Cache miss: executing command...\n\n")

		// run command
		fmt.Print("Running command: ")
		color.Cyan(args.Command)
		commandExecutionStart := time.Now()
		err := internal.RunCommand(args.Command, workingDirectory)

		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		profiling.CommandExecution = time.Since(commandExecutionStart)

		dimGrey.Printf("Command completed in %s\n", formatDuration(profiling.CommandExecution))

		// capture output
		profiling.CacheSaveStart = time.Now()
		outputFiles, err := internal.FindFiles(args.Output, []string{}, workingDirectory)

		if err != nil {
			fmt.Println("error occured while searching for output files")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if len(outputFiles) == 0 {
			fmt.Println("did not find any matching output files, nothing to save!")
			os.Exit(1)
		}

		output, saveOutputErr := internal.CaptureOutput(outputFiles, inputChecksum, workingDirectory)
		if saveOutputErr != nil {
			fmt.Println("error occurred while saving output")
			fmt.Println(saveOutputErr.Error())
		}

		cacheSave(inputChecksum, args.Cache, output)

		printFileList(outputFiles, 10, "+")
	}
}

/* helpers */
func printError(error string, exitCode int) {
	color.Set(color.FgRed)
	fmt.Printf("! ")
	color.Set(color.FgHiWhite)
	fmt.Println(error)
	os.Exit(exitCode)
}

func validateArgs(args RunCommandArgs) {
	// validate input patterns
	for _, inputPattern := range args.Input {
		if strings.Contains(inputPattern, "../") {
			printError("Input pattern cannot be relative, use --cwd to change to parent directory", 1)
		}
	}

	// validate cache providers
	for _, cacheProvider := range args.Cache {
		provider, _ := internal.GetCacheProviderFromURI(cacheProvider)

		if provider == nil {
			printError(fmt.Sprintf("Unknown cache provider: %s", cacheProvider), 1)
		}
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

/* steps */
func cacheLookup(key string, caches []string) io.ReadCloser {
	for _, cacheUri := range caches {
		provider, err := internal.GetCacheProviderFromURI(cacheUri)

		if err != nil {
			dimGrey.Printf("Invalid cache provider: %s\n", cacheUri)
			return nil
		}

		result, err := provider.GetEntry(key)

		if err != nil {
			dimGrey.Printf("Cache miss from %s: %s\n", cacheUri, err.Error())
			continue
		}

		dimGrey.Printf("Cache hit from %s\n", cacheUri)

		return result
	}

	return nil
}

func cacheSave(key string, caches []string, body io.Reader) {
	for _, cacheUri := range caches {
		provider, err := internal.GetCacheProviderFromURI(cacheUri)

		if err != nil {
			dimGrey.Printf("Invalid cache provider: %s\n", cacheUri)
			return
		}

		bytesWritten, err := provider.PutEntry(key, body)

		if err != nil {
			fmt.Println("error occurred while saving cache: ", err.Error())
			os.Exit(1)
		}

		profiling.CacheSave = time.Since(profiling.CacheSaveStart)

		dimGrey.Printf("Stored result (%s) in cache in %s\n", internal.ByteCountSI(bytesWritten), formatDuration(profiling.CacheSave))

		return
	}
}
