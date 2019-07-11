package commands

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/golangci/golangci-lint/pkg/config"
	"github.com/golangci/golangci-lint/pkg/logutils"
)

func (e *Executor) persistentPreRun(_ *cobra.Command, _ []string) {
	if e.cfg.Run.PrintVersion {
		fmt.Fprintf(logutils.StdOut, "golangci-lint has version %s built from %s on %s\n", e.version, e.commit, e.date)
		os.Exit(0)
	}

	runtime.GOMAXPROCS(e.cfg.Run.Concurrency)

	if e.cfg.Run.CPUProfilePath != "" {
		f, err := os.Create(e.cfg.Run.CPUProfilePath)
		if err != nil {
			e.log.Fatalf("Can't create file %s: %s", e.cfg.Run.CPUProfilePath, err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			e.log.Fatalf("Can't start CPU profiling: %s", err)
		}
	}

	if e.cfg.Run.MemProfilePath != "" {
		if rate := os.Getenv("GL_MEMPROFILE_RATE"); rate != "" {
			runtime.MemProfileRate, _ = strconv.Atoi(rate)
		}
	}
}

func (e *Executor) persistentPostRun(_ *cobra.Command, _ []string) {
	if e.cfg.Run.CPUProfilePath != "" {
		pprof.StopCPUProfile()
	}
	if e.cfg.Run.MemProfilePath != "" {
		f, err := os.Create(e.cfg.Run.MemProfilePath)
		if err != nil {
			e.log.Fatalf("Can't create file %s: %s", e.cfg.Run.MemProfilePath, err)
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		printMemStats(&ms, e.log)

		if err := pprof.WriteHeapProfile(f); err != nil {
			e.log.Fatalf("Can't write heap profile: %s", err)
		}
		f.Close()
	}

	os.Exit(e.exitCode)
}

func printMemStats(ms *runtime.MemStats, logger logutils.Log) {
	logger.Infof("Mem stats: alloc=%s total_alloc=%s sys=%s "+
		"heap_alloc=%s heap_sys=%s heap_idle=%s heap_released=%s heap_in_use=%s "+
		"stack_in_use=%s stack_sys=%s "+
		"mspan_sys=%s mcache_sys=%s buck_hash_sys=%s gc_sys=%s other_sys=%s "+
		"mallocs_n=%d frees_n=%d heap_objects_n=%d gc_cpu_fraction=%.2f",
		formatMemory(ms.Alloc), formatMemory(ms.TotalAlloc), formatMemory(ms.Sys),
		formatMemory(ms.HeapAlloc), formatMemory(ms.HeapSys),
		formatMemory(ms.HeapIdle), formatMemory(ms.HeapReleased), formatMemory(ms.HeapInuse),
		formatMemory(ms.StackInuse), formatMemory(ms.StackSys),
		formatMemory(ms.MSpanSys), formatMemory(ms.MCacheSys), formatMemory(ms.BuckHashSys),
		formatMemory(ms.GCSys), formatMemory(ms.OtherSys),
		ms.Mallocs, ms.Frees, ms.HeapObjects, ms.GCCPUFraction)
}

func formatMemory(memBytes uint64) string {
	if memBytes < 1024 {
		return fmt.Sprintf("%db", memBytes)
	}
	if memBytes < 1024*1024 {
		return fmt.Sprintf("%dkb", memBytes/1024)
	}
	return fmt.Sprintf("%dmb", memBytes/1024/1024)
}

func getDefaultConcurrency() int {
	if os.Getenv("HELP_RUN") == "1" {
		return 8 // to make stable concurrency for README help generating builds
	}

	return runtime.NumCPU()
}

func (e *Executor) initRoot() {
	rootCmd := &cobra.Command{
		Use:   "golangci-lint",
		Short: "golangci-lint is a smart linters runner.",
		Long:  `Smart, fast linters runner. Run it in cloud for every GitHub pull request on https://golangci.com`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 0 {
				e.log.Fatalf("Usage: golangci-lint")
			}
			if err := cmd.Help(); err != nil {
				e.log.Fatalf("Can't run help: %s", err)
			}
		},
		PersistentPreRun:  e.persistentPreRun,
		PersistentPostRun: e.persistentPostRun,
	}

	initRootFlagSet(rootCmd.PersistentFlags(), e.cfg, e.needVersionOption())
	e.rootCmd = rootCmd
}

func (e *Executor) needVersionOption() bool {
	return e.date != ""
}

func initRootFlagSet(fs *pflag.FlagSet, cfg *config.Config, needVersionOption bool) {
	fs.BoolVarP(&cfg.Run.IsVerbose, "verbose", "v", false, wh("verbose output"))

	var silent bool
	fs.BoolVarP(&silent, "silent", "s", false, wh("disables congrats outputs"))
	if err := fs.MarkHidden("silent"); err != nil {
		panic(err)
	}
	err := fs.MarkDeprecated("silent",
		"now golangci-lint by default is silent: it doesn't print Congrats message")
	if err != nil {
		panic(err)
	}

	fs.StringVar(&cfg.Run.CPUProfilePath, "cpu-profile-path", "", wh("Path to CPU profile output file"))
	fs.StringVar(&cfg.Run.MemProfilePath, "mem-profile-path", "", wh("Path to memory profile output file"))
	fs.IntVarP(&cfg.Run.Concurrency, "concurrency", "j", getDefaultConcurrency(), wh("Concurrency (default NumCPU)"))
	if needVersionOption {
		fs.BoolVar(&cfg.Run.PrintVersion, "version", false, wh("Print version"))
	}

	fs.StringVar(&cfg.Output.Color, "color", "auto", wh("Use color when printing; can be 'always', 'auto', or 'never'"))
}