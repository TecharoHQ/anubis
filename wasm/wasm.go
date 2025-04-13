package wasm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func UpdateNonce(uint32) {}

type Runner struct {
	r      wazero.Runtime
	code   wazero.CompiledModule
	module api.Module
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

	mod, err := r.InstantiateModule(ctx, code, wazero.NewModuleConfig().WithName(fname))
	if err != nil {
		return nil, fmt.Errorf("wasm: can't instantiate module: %w", err)
	}

	result := &Runner{
		r:      r,
		code:   code,
		module: mod,
	}

	if err := result.checkExports(); err != nil {
		return nil, fmt.Errorf("wasm: module is missing exports: %w", err)
	}

	return result, nil
}

func (r *Runner) checkExports() error {
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
		if r.module.ExportedFunction(fun) == nil {
			errs = append(errs, fmt.Errorf("function %s is not defined", fun))
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (r *Runner) anubisWork(ctx context.Context, difficulty, initialNonce, iterand uint32) (uint32, error) {
	results, err := r.module.ExportedFunction("anubis_work").Call(ctx, uint64(difficulty), uint64(initialNonce), uint64(iterand))
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) anubisValidate(ctx context.Context, nonce, difficulty uint32) (bool, error) {
	results, err := r.module.ExportedFunction("anubis_validate").Call(ctx, uint64(nonce), uint64(difficulty))
	if err != nil {
		return false, err
	}

	// Rust booleans are 1 if true
	return results[0] == 1, nil
}

func (r *Runner) dataPtr(ctx context.Context) (uint32, error) {
	results, err := r.module.ExportedFunction("data_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) setDataLength(ctx context.Context, length uint32) error {
	_, err := r.module.ExportedFunction("set_data_length").Call(ctx, uint64(length))
	return err
}

func (r *Runner) resultHashPtr(ctx context.Context) (uint32, error) {
	results, err := r.module.ExportedFunction("result_hash_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) resultHashSize(ctx context.Context) (uint32, error) {
	results, err := r.module.ExportedFunction("result_hash_size").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) verificationHashPtr(ctx context.Context) (uint32, error) {
	results, err := r.module.ExportedFunction("verification_hash_ptr").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) verificationHashSize(ctx context.Context) (uint32, error) {
	results, err := r.module.ExportedFunction("verification_hash_size").Call(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(results[0]), nil
}

func (r *Runner) WriteData(ctx context.Context, data []byte) (uint32, error) {
	if len(data) > 4096 {
		return 0, os.ErrInvalid
	}

	length := uint32(len(data))

	dataPtr, err := r.dataPtr(ctx)
	if err != nil {
		return 0, fmt.Errorf("can't read data pointer: %w", err)
	}

	if !r.module.Memory().Write(dataPtr, data) {
		return 0, fmt.Errorf("[unexpected] can't write memory, is data out of range??")
	}

	if err := r.setDataLength(ctx, length); err != nil {
		return 0, fmt.Errorf("can't set data length: %w", err)
	}

	return length, nil
}

func (r *Runner) ReadResult(ctx context.Context) ([]byte, error) {
	length, err := r.resultHashSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get result hash size: %w", err)
	}

	ptr, err := r.resultHashPtr(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get result hash pointer: %w", err)
	}

	buf, ok := r.module.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("[unexpected] can't read from memory, is something out of range??")
	}

	return buf, nil
}

func (r *Runner) WriteVerification(ctx context.Context, data []byte) error {
	length, err := r.verificationHashSize(ctx)
	if err != nil {
		return fmt.Errorf("can't get verification hash size: %v", err)
	}

	if length != uint32(len(data)) {
		return fmt.Errorf("data is too big, want %d bytes, got: %d", length, len(data))
	}

	ptr, err := r.verificationHashPtr(ctx)
	if err != nil {
		return fmt.Errorf("can't get verification hash pointer: %v", err)
	}

	if !r.module.Memory().Write(ptr, data) {
		return fmt.Errorf("[unexpected] can't write memory, is data out of range??")
	}

	return nil
}
