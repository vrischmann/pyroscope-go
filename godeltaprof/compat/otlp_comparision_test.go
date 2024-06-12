//go:build go1.19

package compat

import (
	"bytes"
	"fmt"
	gprofile "github.com/google/pprof/profile"
	"github.com/grafana/pyroscope-go/godeltaprof"
	"github.com/grafana/pyroscope-go/godeltaprof/otlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otlpprofile "go.opentelemetry.io/proto/otlp/profiles/v1experimental"
	"google.golang.org/protobuf/proto"
	"runtime"
	"sort"
	"testing"
)

func TestOTLPHeap(t *testing.T) {
	h := newHeapTestHelper()
	fs := h.generateMemProfileRecords(512, 32)
	h.rng.Seed(239)
	nmutations := int(h.rng.Int63() % int64(len(fs)))
	otlp := otlp.NewHeapProfilerWithOptions(godeltaprof.ProfileOptions{
		GenericsFrames: true,
		LazyMappings:   true,
	})

	for i := 0; i < 1024; i++ {
		if i == 1000 {
			v := h.rng.Int63()
			if v != 7817861117094116717 {
				t.Errorf("unexpected random value: %d. "+
					"The bench should be deterministic for better comparision.", v)
			}
		}
		p1 := bytes.NewBuffer(nil)
		err := WriteHeapProto(h.dp, h.opt, p1, fs, int64(runtime.MemProfileRate))
		assert.NoError(t, err)

		p2, err := otlp.ProfileFromRecords(fs)
		assert.NoError(t, err)

		compareOTLP(t, p1, p2)

		h.mutate(nmutations, fs)
	}

}

func TestOTLPMutex(t *testing.T) {
	period := runtime.SetMutexProfileFraction(-1)
	defer runtime.SetMutexProfileFraction(period)
	runtime.SetMutexProfileFraction(1)
	h := newMutexTestHelper()
	fs := h.generateBlockProfileRecords(512, 32)
	h.rng.Seed(239)
	nmutations := int(h.rng.Int63() % int64(len(fs)))
	opt := godeltaprof.ProfileOptions{
		GenericsFrames: true,
		LazyMappings:   true,
	}
	otlp := otlp.NewMutexProfilerWithOptions(opt)
	for i := 0; i < 1024; i++ {
		if i == 1000 {
			v := h.rng.Int63()
			if v != 7817861117094116717 {
				t.Errorf("unexpected random value: %d. "+
					"The bench should be deterministic for better comparision.", v)
			}
		}
		p1 := bytes.NewBuffer(nil)
		err := PrintCountCycleProfile(h.dp, h.opt, p1, h.scaler, fs)
		assert.NoError(t, err)

		p2, err := otlp.ProfileFromRecords(fs)
		assert.NoError(t, err)

		compareOTLP(t, p1, p2)

		h.mutate(nmutations, fs)
	}
}

func compareOTLP(t *testing.T, pprofBytes *bytes.Buffer, otlp *otlpprofile.Profile) {
	pprof, err := gprofile.ParseData(pprofBytes.Bytes())
	require.NoError(t, err)
	pprofSamples := make([]string, 0, len(pprof.Sample))
	for _, s := range pprof.Sample {
		assert.GreaterOrEqual(t, len(s.Value), 2)
		pprofSamples = append(pprofSamples, fmt.Sprintf("%s %+v", pprofSampleStackToString(s), s.Value))
	}
	sort.Slice(pprofSamples, func(i, j int) bool { return pprofSamples[i] < pprofSamples[j] })

	otlpSamples := make([]string, 0, len(otlp.Sample))
	for _, s := range otlp.Sample {
		assert.GreaterOrEqual(t, len(s.Value), 2)
		otlpSamples = append(otlpSamples, fmt.Sprintf("%s %+v", otlpSampleStackToString(otlp, s), s.Value))
	}
	sort.Slice(otlpSamples, func(i, j int) bool { return otlpSamples[i] < otlpSamples[j] })
	require.Equal(t, pprofSamples, otlpSamples)
	require.Greater(t, len(pprofSamples), 0)
}

func BenchmarkPPROFNoCompression(b *testing.B) {
	h := newHeapTestHelper()
	h.opt.NoCompression = true
	fs := h.generateMemProfileRecords(512, 32)
	h.rng.Seed(239)
	nmutations := int(h.rng.Int63() % int64(len(fs)))

	b.ResetTimer()
	p1 := bytes.NewBuffer(nil)

	for i := 0; i < b.N; i++ {
		if i == 1000 {
			v := h.rng.Int63()
			if v != 7817861117094116717 {
				b.Errorf("unexpected random value: %d. "+
					"The bench should be deterministic for better comparision.", v)
			}
		}
		p1.Reset()
		_ = WriteHeapProto(h.dp, h.opt, p1, fs, int64(runtime.MemProfileRate))

		h.mutate(nmutations, fs)
	}
}

func BenchmarkOTLP(b *testing.B) {
	h := newHeapTestHelper()
	fs := h.generateMemProfileRecords(512, 32)
	h.rng.Seed(239)
	nmutations := int(h.rng.Int63() % int64(len(fs)))
	otlp := otlp.NewHeapProfilerWithOptions(godeltaprof.ProfileOptions{
		GenericsFrames: true,
		LazyMappings:   true,
	})
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if i == 1000 {
			v := h.rng.Int63()
			if v != 7817861117094116717 {
				b.Errorf("unexpected random value: %d. "+
					"The bench should be deterministic for better comparision.", v)
			}
		}

		p, _ := otlp.ProfileFromRecords(fs)
		_, _ = proto.Marshal(p)

		h.mutate(nmutations, fs)
	}
}