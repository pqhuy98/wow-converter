package bench

import (
	"strconv"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
)

const (
	benchTextureCount = 32
	benchPNGSize       = 128
	benchChunkJobs     = 256
	benchTileJobs      = 16
)

func BenchmarkWorkerPool(b *testing.B) {
	tasks := make([]func() error, benchTextureCount)
	for i := range tasks {
		i := i
		tasks[i] = func() error {
			cpuWork(i)
			return nil
		}
	}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(1, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(benchConcurrency(), tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkFormatsQueue(b *testing.B) {
	items := make([]int, benchTextureCount)
	for i := range items {
		items[i] = i
	}
	handler := func(seed int) error {
		cpuWork(seed)
		return nil
	}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := formats.Queue(items, handler, 1); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := formats.Queue(items, handler, benchConcurrency()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBLPTextureExport(b *testing.B) {
	png, err := syntheticPNG(benchPNGSize)
	if err != nil {
		b.Fatalf("synthetic png: %v", err)
	}
	pngs := make([][]byte, benchTextureCount)
	for i := range pngs {
		pngs[i] = png
	}
	dir := b.TempDir()
	blp.EnsureWorkerPool(0)
	workers := blp.GetWorkerPoolSize()
	if workers > len(pngs) {
		workers = len(pngs)
	}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, data := range pngs {
				out := dir + "/seq_" + strconv.Itoa(i) + "_" + strconv.Itoa(j) + ".blp"
				if err := blp.ConvertTextureToBlp(blp.EncodeInput{PNG: data}, out); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tasks := make([]func() error, len(pngs))
			for j, data := range pngs {
				j, data := j, data
				out := dir + "/par_" + strconv.Itoa(i) + "_" + strconv.Itoa(j) + ".blp"
				tasks[j] = func() error {
					return blp.SubmitBlpTask(blp.TaskInput{Kind: "png", Data: data}, out)
				}
			}
			if err := common.WorkerPool(workers, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkADTChunkBake(b *testing.B) {
	jobs := syntheticChunkBakeJobs(benchChunkJobs)

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, job := range jobs {
				adt.BakeChunk(job)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := formats.Queue(jobs, func(job adt.ChunkBakeParams) error {
				adt.BakeChunk(job)
				return nil
			}, config.MaxConcurrency()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTileExportWorkers(b *testing.B) {
	tasks := make([]func() error, benchTileJobs)
	for i := range tasks {
		i := i
		tasks[i] = func() error {
			cpuWork(i + 1000)
			return nil
		}
	}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(1, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(mapexporter.MapExportWorkerCount(), tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkIconBLPExport(b *testing.B) {
	png, err := syntheticPNG(64)
	if err != nil {
		b.Fatalf("synthetic png: %v", err)
	}
	pngs := make([][]byte, benchTextureCount)
	for i := range pngs {
		pngs[i] = png
	}
	dir := b.TempDir()
	blp.EnsureWorkerPool(0)
	workers := blp.GetWorkerPoolSize()
	if workers > len(pngs) {
		workers = len(pngs)
	}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, data := range pngs {
				out := dir + "/icon_seq_" + strconv.Itoa(i) + "_" + strconv.Itoa(j) + ".blp"
				if err := blp.ConvertTextureToBlp(blp.EncodeInput{PNG: data}, out); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tasks := make([]func() error, len(pngs))
			for j, data := range pngs {
				j, data := j, data
				out := dir + "/icon_par_" + strconv.Itoa(i) + "_" + strconv.Itoa(j) + ".blp"
				tasks[j] = func() error {
					return blp.SubmitBlpTask(blp.TaskInput{Kind: "png", Data: data}, out)
				}
			}
			if err := common.WorkerPool(workers, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCreatureExportWorkers(b *testing.B) {
	tasks := make([]func() error, benchTileJobs)
	for i := range tasks {
		i := i
		tasks[i] = func() error {
			cpuWork(i + 2000)
			return nil
		}
	}
	creatureWorkers := mapexporter.MapExportWorkerCount()

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(1, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := common.WorkerPool(creatureWorkers, tasks); err != nil {
				b.Fatal(err)
			}
		}
	})
}
