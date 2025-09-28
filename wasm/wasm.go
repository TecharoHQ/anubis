package wasm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func UpdateNonce(uint32) {}

var (
	validationTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "anubis_wasm_validation_time",
		Help:    "The time taken for the validation function to run per checker (nanoseconds)",
		Buckets: prometheus.ExponentialBucketsRange(1, math.Pow(2, 31), 32),
	}, []string{"fname"})

	validationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_wasm_validation",
		Help: "The number of times the validation logic has been run and its success rate",
	}, []string{"fname", "success"})
)

type Runner struct {
	r     wazero.Runtime
	code  wazero.CompiledModule
	fname string
	lock  sync.Mutex
}

func NewRunner(ctx context.Context, fname string, fin io.ReadCloser) (*Runner, error) {
	data, err := io.ReadAll(fin)
	if err != nil {
		return nil, fmt.Errorf("wasm: can't read from fin: %w", err)
	}

	r := wazero.NewRuntime(ctx)

	_, err = r.NewHostModuleBuilder("anubis").
		NewFunctionBuilder().
		WithFunc(func(context.Context, uint32) {}).
		Export("anubis_update_nonce").
		Instantiate(ctx)
	if err != nil {
		return nil, fmt.Errorf("wasm: can't export anubis_update_nonce: %w", err)
	}

	code, err := r.CompileModule(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("wasm: can't compile module: %w", err)
	}

	result := &Runner{
		r:     r,
		code:  code,
		fname: fname,
	}

	return result, nil
}

func (r *Runner) checkExports(module api.Module) error {
	funcs := []string{
		"anubis_work",
		"anubis_validate",
		"data_ptr",
		"set_data_length",
		"result_hash_ptr",
		"result_hash_size",
		"verification_hash_ptr",
		"verification_hash_size",
	}

	var errs []error

	for _, fun := range funcs {
		if module.ExportedFunction(fun) == nil {
			errs = append(errs, fmt.Errorf("function %s is not defined", fun))
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (r *Runner) anubisWork(ctx context.Context, module api.Module, difficulty, initialNonce, iterand uint32) (uint32, error) {
	results, err := module.ExportedFunction("anubis_work").Call(ctx, uint64(difficulty), uint64(initialNonce), uint64(iterand))
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) anubisValidate(ctx context.Context, module api.Module, nonce, difficulty uint32) (bool, error) {
	results, err := module.ExportedFunction("anubis_validate").Call(ctx, uint64(nonce), uint64(difficulty))
	if err != nil {
		return false, err
	}

	// Rust booleans are 1 if true
	return results[0] == 1, nil
}

func (r *Runner) dataPtr(ctx context.Context, module api.Module) (uint32, error) {
	results, err := module.ExportedFunction("data_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) setDataLength(ctx context.Context, module api.Module, length uint32) error {
	_, err := module.ExportedFunction("set_data_length").Call(ctx, uint64(length))
	return err
}

func (r *Runner) resultHashPtr(ctx context.Context, module api.Module) (uint32, error) {
	results, err := module.ExportedFunction("result_hash_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) resultHashSize(ctx context.Context, module api.Module) (uint32, error) {
	results, err := module.ExportedFunction("result_hash_size").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) verificationHashPtr(ctx context.Context, module api.Module) (uint32, error) {
	results, err := module.ExportedFunction("verification_hash_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) verificationHashSize(ctx context.Context, module api.Module) (uint32, error) {
	results, err := module.ExportedFunction("verification_hash_size").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) writeData(ctx context.Context, module api.Module, data []byte) error {
	if len(data) > 4096 {
		return os.ErrInvalid
	}

	length := uint32(len(data))

	dataPtr, err := r.dataPtr(ctx, module)
	if err != nil {
		return fmt.Errorf("can't read data pointer: %w", err)
	}

	if !module.Memory().Write(dataPtr, data) {
		return fmt.Errorf("[unexpected] can't write memory, is data out of range??")
	}

	if err := r.setDataLength(ctx, module, length); err != nil {
		return fmt.Errorf("can't set data length: %w", err)
	}

	return nil
}

func (r *Runner) readResult(ctx context.Context, module api.Module) ([]byte, error) {
	length, err := r.resultHashSize(ctx, module)
	if err != nil {
		return nil, fmt.Errorf("can't get result hash size: %w", err)
	}

	ptr, err := r.resultHashPtr(ctx, module)
	if err != nil {
		return nil, fmt.Errorf("can't get result hash pointer: %w", err)
	}

	buf, ok := module.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("[unexpected] can't read from memory, is something out of range??")
	}

	return buf, nil
}

func (r *Runner) run(ctx context.Context, data []byte, difficulty, initialNonce, iterand uint32) (uint32, []byte, api.Module, error) {
	mod, err := r.r.InstantiateModule(ctx, r.code, wazero.NewModuleConfig().WithName(r.fname))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("can't instantiate module: %w", err)
	}

	if err := r.checkExports(mod); err != nil {
		return 0, nil, nil, err
	}

	if err := r.writeData(ctx, mod, data); err != nil {
		return 0, nil, nil, err
	}

	nonce, err := r.anubisWork(ctx, mod, difficulty, initialNonce, iterand)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("can't run work function: %w", err)
	}

	hash, err := r.readResult(ctx, mod)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("can't read result: %w", err)
	}

	return nonce, hash, mod, nil
}

func (r *Runner) Run(ctx context.Context, data []byte, difficulty, initialNonce, iterand uint32) (uint32, []byte, error) {
	nonce, hash, _, err := r.run(ctx, data, difficulty, initialNonce, iterand)
	if err != nil {
		return 0, nil, fmt.Errorf("can't run %s: %w", r.fname, err)
	}

	return nonce, hash, nil
}

func (r *Runner) verify(ctx context.Context, data, verify []byte, nonce, difficulty uint32) (bool, api.Module, error) {
	mod, err := r.r.InstantiateModule(ctx, r.code, wazero.NewModuleConfig().WithName(r.fname))
	if err != nil {
		return false, nil, fmt.Errorf("can't instantiate module: %w", err)
	}

	if err := r.checkExports(mod); err != nil {
		return false, nil, err
	}

	if err := r.writeData(ctx, mod, data); err != nil {
		return false, nil, err
	}

	if err := r.writeVerification(ctx, mod, verify); err != nil {
		return false, nil, err
	}

	ok, err := r.anubisValidate(ctx, mod, nonce, difficulty)
	if err != nil {
		return false, nil, fmt.Errorf("can't validate hash %x from challenge %x, nonce %d and difficulty %d: %w", verify, data, nonce, difficulty, err)
	}

	return ok, mod, nil
}

func (r *Runner) Verify(ctx context.Context, data, verify []byte, nonce, difficulty uint32) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	t0 := time.Now()
	ok, _, err := r.verify(ctx, data, verify, nonce, difficulty)
	validationTime.WithLabelValues(r.fname).Observe(float64(time.Since(t0)))
	validationCount.WithLabelValues(r.fname, strconv.FormatBool(ok))
	return ok, err
}

func (r *Runner) writeVerification(ctx context.Context, module api.Module, data []byte) error {
	length, err := r.verificationHashSize(ctx, module)
	if err != nil {
		return fmt.Errorf("can't get verification hash size: %v", err)
	}

	if length != uint32(len(data)) {
		return fmt.Errorf("data is too big, want %d bytes, got: %d", length, len(data))
	}

	ptr, err := r.verificationHashPtr(ctx, module)
	if err != nil {
		return fmt.Errorf("can't get verification hash pointer: %v", err)
	}

	if !module.Memory().Write(ptr, data) {
		return fmt.Errorf("[unexpected] can't write memory, is data out of range??")
	}

	return nil
}
