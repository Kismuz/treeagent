package treeagent

import (
	"github.com/unixpickle/anyrl"
	"github.com/unixpickle/anyvec"
)

// Sample is a training sample for building a tree.
//
// Each Sample maps an observation and an action to an
// advantage approximator of some kind.
type Sample interface {
	Feature(idx int) float64
	Action() int
	Advantage() float64
}

// RolloutSamples produces a stream of Samples based on
// the batch of rollouts.
// Each Sample represents a single timestep.
// The advantages can come from an anypg.ActionJudger.
//
// The resulting channel is sorted first by timestep and
// then by index in the batch.
// Thus, Samples from time t are always before Samples
// from time t+1.
//
// The caller must read the entire channel to prevent a
// resource leak.
func RolloutSamples(r *anyrl.RolloutSet, advantages anyrl.Rewards) <-chan Sample {
	res := make(chan Sample, 1)
	go func() {
		defer close(res)
		inChan := r.Inputs.ReadTape(0, -1)
		outChan := r.Actions.ReadTape(0, -1)
		timestep := 0
		for input := range inChan {
			output := <-outChan
			inValues := vecToFloats(input.Packed)

			batch := input.NumPresent()
			numFeatures := len(inValues) / batch
			numActions := output.Packed.Len() / batch
			i := 0
			for lane, pres := range input.Present {
				if !pres {
					continue
				}
				subIns := inValues[i*numFeatures : (i+1)*numFeatures]
				subOuts := output.Packed.Slice(i*numActions, (i+1)*numActions)
				action := anyvec.MaxIndex(subOuts)
				res <- &memorySample{
					features:  subIns,
					action:    action,
					advantage: advantages[lane][timestep],
				}
				i++
			}
			timestep++
		}
	}()
	return res
}

// Uint8Samples shrinks the memory footprint of a Sample
// stream by storing the features as uint8 values.
//
// The order of the input channel is preserved in the
// output channel.
//
// You should only use this if you know that the features
// are 8-bit integers.
//
// The caller must read the entire channel to prevent a
// resource leak.
// Doing so will automatically read the incoming channel
// in its entirety.
func Uint8Samples(numFeatures int, incoming <-chan Sample) <-chan Sample {
	res := make(chan Sample, 1)
	go func() {
		defer close(res)
		for in := range incoming {
			sample := &uint8Sample{
				features:  make([]uint8, numFeatures),
				action:    in.Action(),
				advantage: in.Advantage(),
			}
			for i := 0; i < numFeatures; i++ {
				sample.features[i] = uint8(in.Feature(i))
			}
			res <- sample
		}
	}()
	return res
}

// AllSamples reads the samples from the channel and
// stores them in a slice.
func AllSamples(ch <-chan Sample) []Sample {
	var res []Sample
	for s := range ch {
		res = append(res, s)
	}
	return res
}

type memorySample struct {
	features  []float64
	action    int
	advantage float64
}

func (m *memorySample) Feature(idx int) float64 {
	return m.features[idx]
}

func (m *memorySample) Action() int {
	return m.action
}

func (m *memorySample) Advantage() float64 {
	return m.advantage
}

type uint8Sample struct {
	features  []uint8
	action    int
	advantage float64
}

func (u *uint8Sample) Feature(idx int) float64 {
	return float64(u.features[idx])
}

func (u *uint8Sample) Action() int {
	return u.action
}

func (u *uint8Sample) Advantage() float64 {
	return u.advantage
}
