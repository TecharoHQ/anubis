import { useEffect, useRef, useState } from "react";
import clsx from "clsx";
import styles from "./styles.module.css";

// The diagram shows a 64 bit hash rather than a full SHA-256 digest. Real
// hashes are 256 bits, but 64 of them fit on a phone screen and the point
// about bits versus nibbles is the same either way.
const HASH_BYTES = 8;
const HEX_CHARS = HASH_BYTES * 2;
const HASH_BITS = HASH_BYTES * 8;

// Each row of the diagram covers four hex characters, so 16 bits.
const HEX_PER_CHUNK = 4;
const BITS_PER_CHUNK = HEX_PER_CHUNK * 4;
const CHUNKS = HEX_CHARS / HEX_PER_CHUNK;

// A fixed starting hash so the server-rendered markup matches the first
// client render. Mining replaces it with random hashes.
const INITIAL_HEX = "f3a2b1c4d5e6f7a8";
const INITIAL_BIN = hexToBin(INITIAL_HEX);

// Number of hashes to try per animation frame. High enough to feel like
// mining, low enough to keep the page responsive.
const HASHES_PER_FRAME = 200;

// How often the hash display is allowed to change when the reader has asked
// for reduced motion. Mining still runs every frame; only the repaint slows
// down, which turns a 60fps strobe into a readable ticker.
const REDUCED_MOTION_PAINT_MS = 400;

// Converts a hex string into its binary expansion, four bits per character.
function hexToBin(hex) {
  return hex
    .split("")
    .map((c) => parseInt(c, 16).toString(2).padStart(4, "0"))
    .join("");
}

// Generates a random hash value in both hex and binary form.
function generateRandomHash() {
  const buffer = new Uint8Array(HASH_BYTES);
  crypto.getRandomValues(buffer);
  const hex = Array.from(buffer)
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
  return { hex, bin: hexToBin(hex) };
}

// Reports whether a hash clears the target: `difficulty` leading zero bits in
// bit mode, or `difficulty` leading zero hex characters in nibble mode.
function meetsTarget(mode, difficulty, hex, bin) {
  const prefix = "0".repeat(difficulty);
  return mode === "bit" ? bin.startsWith(prefix) : hex.startsWith(prefix);
}

// Tracks the reader's reduced motion preference. Starts false so the server
// and the first client render agree, then corrects itself on mount.
function usePrefersReducedMotion() {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    setPrefersReducedMotion(query.matches);

    const handleChange = (event) => setPrefersReducedMotion(event.matches);
    query.addEventListener("change", handleChange);
    return () => query.removeEventListener("change", handleChange);
  }, []);

  return prefersReducedMotion;
}

export default function DifficultyVisualizer() {
  const [mode, setMode] = useState("nibble");
  const [difficulty, setDifficulty] = useState(1);
  const [nonce, setNonce] = useState(0);
  const [hashHex, setHashHex] = useState(INITIAL_HEX);
  const [hashBin, setHashBin] = useState(INITIAL_BIN);
  const [isMining, setIsMining] = useState(false);
  const frameRef = useRef(null);
  const untalliedRef = useRef(0);
  const lastPaintRef = useRef(0);
  const prefersReducedMotion = usePrefersReducedMotion();

  const maxDifficulty = mode === "bit" ? HASH_BITS : HEX_CHARS;
  const isSolved = nonce > 0 && meetsTarget(mode, difficulty, hashHex, hashBin);
  const isSlow =
    (mode === "bit" && difficulty > 20) ||
    (mode === "nibble" && difficulty > 5);

  // Drives the mining loop. Reruns whenever the target changes so the loop
  // always tests against the current mode and difficulty.
  useEffect(() => {
    if (!isMining) return;

    const mineStep = () => {
      let found = false;
      let latest = null;

      for (let i = 0; i < HASHES_PER_FRAME; i++) {
        untalliedRef.current++;
        latest = generateRandomHash();
        if (meetsTarget(mode, difficulty, latest.hex, latest.bin)) {
          found = true;
          break;
        }
      }

      // Under reduced motion, keep hashing at full speed but let the display
      // settle between updates. The winning hash always paints immediately.
      const now = performance.now();
      const shouldPaint =
        found ||
        !prefersReducedMotion ||
        now - lastPaintRef.current >= REDUCED_MOTION_PAINT_MS;

      if (shouldPaint) {
        const tried = untalliedRef.current;
        untalliedRef.current = 0;
        lastPaintRef.current = now;
        setNonce((prev) => prev + tried);
        setHashHex(latest.hex);
        setHashBin(latest.bin);
      }

      if (found) {
        setIsMining(false);
      } else {
        frameRef.current = requestAnimationFrame(mineStep);
      }
    };

    frameRef.current = requestAnimationFrame(mineStep);

    return () => {
      if (frameRef.current !== null) {
        cancelAnimationFrame(frameRef.current);
        frameRef.current = null;
      }

      // Stopping between throttled repaints would otherwise drop the hashes
      // tried since the last one, leaving the nonce reading low.
      if (untalliedRef.current > 0) {
        const tried = untalliedRef.current;
        untalliedRef.current = 0;
        setNonce((prev) => prev + tried);
      }
    };
  }, [isMining, mode, difficulty, prefersReducedMotion]);

  const handleModeChange = (nextMode) => {
    setMode(nextMode);
    setDifficulty(1);
    setIsMining(false);
  };

  const handleDifficultyChange = (event) => {
    setDifficulty(Number(event.target.value));
    setIsMining(false);
  };

  const handleMineClick = () => {
    setNonce(0);
    untalliedRef.current = 0;
    lastPaintRef.current = 0;
    setIsMining(true);
  };

  const handleStopClick = () => {
    setIsMining(false);
  };

  // Renders one slice of the hash: a few hex characters with the bits they
  // encode lined up directly underneath them.
  const renderChunk = (chunkIndex) => {
    const hexOffset = chunkIndex * HEX_PER_CHUNK;
    const binOffset = chunkIndex * BITS_PER_CHUNK;
    const hexChunk = hashHex.slice(hexOffset, hexOffset + HEX_PER_CHUNK);
    const binChunk = hashBin.slice(binOffset, binOffset + BITS_PER_CHUNK);

    return (
      <div key={chunkIndex} className={styles.chunk}>
        {chunkIndex === 0 && (
          <div className={styles.sectionLabel}>Hexadecimal</div>
        )}
        <div className={styles.hexRow}>
          {hexChunk.split("").map((char, localIndex) => {
            const index = hexOffset + localIndex;
            const isTarget = mode === "nibble" && index < difficulty;
            return (
              <div
                key={`hex-${index}`}
                className={clsx(
                  styles.hexCell,
                  isTarget && (char === "0" ? styles.cellHit : styles.cellBad),
                )}
              >
                {char}
              </div>
            );
          })}
        </div>

        {chunkIndex === 0 && (
          <div className={styles.sectionLabel}>Binary ({HASH_BITS} bits)</div>
        )}
        <div className={styles.binRow}>
          {binChunk.split("").map((bit, localIndex) => {
            const index = binOffset + localIndex;
            const isBitTarget = mode === "bit" && index < difficulty;
            const isNibbleTarget =
              mode === "nibble" && Math.floor(index / 4) < difficulty;
            const isZero = bit === "0";

            let state = null;
            if (isBitTarget) {
              state = isZero ? styles.cellOk : styles.cellBad;
            } else if (isNibbleTarget) {
              state = isZero ? styles.cellHit : styles.cellBad;
            }

            return (
              <div key={`bin-${index}`} className={clsx(styles.binCell, state)}>
                {bit}
              </div>
            );
          })}
        </div>
      </div>
    );
  };

  return (
    <div className={styles.container}>
      {/* Controls */}
      <div className={styles.controls}>
        <div className={styles.controlGroup}>
          <span className={styles.label}>Measurement mode</span>
          <div className={styles.modeButtons}>
            <button
              type="button"
              onClick={() => handleModeChange("nibble")}
              className={clsx(
                styles.modeButton,
                mode === "nibble" && styles.modeButtonActiveHit,
              )}
            >
              Nibbles (hex)
            </button>
            <button
              type="button"
              onClick={() => handleModeChange("bit")}
              className={clsx(
                styles.modeButton,
                mode === "bit" && styles.modeButtonActiveOk,
              )}
            >
              Bits (binary)
            </button>
          </div>
        </div>

        <div className={styles.controlGroup}>
          <label
            htmlFor="dv-difficulty"
            className={clsx(styles.label, styles.labelRow)}
          >
            <span>Target difficulty</span>
            <span className={styles.difficultyValue}>
              {difficulty} {mode}
              {difficulty === 1 ? "" : "s"}
            </span>
          </label>
          <input
            id="dv-difficulty"
            type="range"
            min="1"
            max={maxDifficulty}
            value={difficulty}
            onChange={handleDifficultyChange}
            className={styles.slider}
          />
          <div className={styles.hintRow}>
            <p className={styles.hint}>
              {mode === "nibble" ? difficulty * 4 : difficulty} / {HASH_BITS}{" "}
              leading zero bits
            </p>
            {isSlow && <p className={styles.warning}>Slow</p>}
          </div>
        </div>
      </div>

      {/* Mining dashboard */}
      <div className={styles.dashboard}>
        {!isMining && isSolved && <div className={styles.successGlow} />}

        <div className={styles.statusRow}>
          <div className={styles.nonce}>
            Nonce:
            <span className={styles.nonceValue}>{nonce.toLocaleString()}</span>
          </div>
          <button
            type="button"
            onClick={isMining ? handleStopClick : handleMineClick}
            className={clsx(
              styles.mineButton,
              isMining && styles.mineButtonMining,
            )}
          >
            {isMining ? "Stop mining" : "Start mining"}
          </button>
        </div>

        <div className={styles.hashGrid}>
          {Array.from({ length: CHUNKS }, (_, i) => renderChunk(i))}
        </div>
      </div>
    </div>
  );
}
