import React, { useState, useEffect, useMemo } from 'react';
import styles from './styles.module.css';

// A helper function to perform SHA-256 hashing.
// It takes a string, encodes it, hashes it, and returns a hex string.
async function sha256(message) {
  try {
    const msgBuffer = new TextEncoder().encode(message);
    const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    return hashHex;
  } catch (error) {
    console.error("Hashing failed:", error);
    return "Error hashing data";
  }
}

// Generates a random hex string of a given byte length
const generateRandomHex = (bytes = 16) => {
  const buffer = new Uint8Array(bytes);
  crypto.getRandomValues(buffer);
  return Array.from(buffer)
    .map(byte => byte.toString(16).padStart(2, '0'))
    .join('');
};


// Icon components for better visual feedback
const CheckIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" className={styles.iconGreen} fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

const XCircleIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" className={styles.iconRed} fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

// Main Application Component
export default function App() {
  // State for the challenge, initialized with a random 16-byte hex string.
  const [challenge, setChallenge] = useState(() => generateRandomHex(16));
  // State for the nonce, which is the variable we can change
  const [nonce, setNonce] = useState(0);
  // State to store the resulting hash
  const [hash, setHash] = useState('');
  // A flag to indicate if the current hash is the "winning" one
  const [isMining, setIsMining] = useState(false);
  const [isFound, setIsFound] = useState(false);

  // The mining difficulty, i.e., the required number of leading zeros
  const difficulty = "00";

  // Memoize the combined data to avoid recalculating on every render
  const combinedData = useMemo(() => `${challenge}${nonce}`, [challenge, nonce]);

  // This effect hook recalculates the hash whenever the combinedData changes.
  useEffect(() => {
    let isMounted = true;
    const calculateHash = async () => {
      const calculatedHash = await sha256(combinedData);
      if (isMounted) {
        setHash(calculatedHash);
        setIsFound(calculatedHash.startsWith(difficulty));
      }
    };
    calculateHash();
    return () => { isMounted = false; };
  }, [combinedData, difficulty]);

  // This effect handles the automatic mining process
  useEffect(() => {
    if (!isMining) return;

    let miningNonce = nonce;
    let continueMining = true;

    const mine = async () => {
      while (continueMining) {
        const currentData = `${challenge}${miningNonce}`;
        const currentHash = await sha256(currentData);

        if (currentHash.startsWith(difficulty)) {
          setNonce(miningNonce);
          setIsMining(false);
          break;
        }

        miningNonce++;
        // Update the UI periodically to avoid freezing the browser
        if (miningNonce % 100 === 0) {
          setNonce(miningNonce);
          await new Promise(resolve => setTimeout(resolve, 0)); // Yield to the browser
        }
      }
    };

    mine();

    return () => {
      continueMining = false;
    }
  }, [isMining, challenge, nonce, difficulty]);


  const handleMineClick = () => {
    setIsMining(true);
  }

  const handleStopClick = () => {
    setIsMining(false);
  }

  const handleResetClick = () => {
    setIsMining(false);
    setNonce(0);
  }

  const handleNewChallengeClick = () => {
    setIsMining(false);
    setChallenge(generateRandomHex(16));
    setNonce(0);
  }

  // Helper to render the hash with colored leading characters
  const renderHash = () => {
    if (!hash) return <span>...</span>;
    const prefix = hash.substring(0, difficulty.length);
    const suffix = hash.substring(difficulty.length);
    const prefixColor = isFound ? styles.hashPrefixGreen : styles.hashPrefixRed;
    return (
      <>
        <span className={`${prefixColor} ${styles.hashPrefix}`}>{prefix}</span>
        <span className={styles.hashSuffix}>{suffix}</span>
      </>
    );
  };

  return (
    <div className={styles.container}>
      <div className={styles.innerContainer}>
        <div className={styles.grid}>
          {/* Challenge Block */}
          <div className={styles.block}>
            <h2 className={styles.blockTitle}>1. Challenge</h2>
            <p className={styles.challengeText}>{challenge}</p>
          </div>

          {/* Nonce Control Block */}
          <div className={styles.block}>
            <h2 className={styles.blockTitle}>2. Nonce</h2>
            <div className={styles.nonceControls}>
              <button onClick={() => setNonce(n => n - 1)} disabled={isMining} className={styles.nonceButton}>
                <svg xmlns="http://www.w3.org/2000/svg" className={styles.iconSmall} fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 12H4" /></svg>
              </button>
              <span className={styles.nonceValue}>{nonce}</span>
              <button onClick={() => setNonce(n => n + 1)} disabled={isMining} className={styles.nonceButton}>
                <svg xmlns="http://www.w3.org/2000/svg" className={styles.iconSmall} fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
              </button>
            </div>
          </div>

          {/* Combined Data Block */}
          <div className={styles.block}>
            <h2 className={styles.blockTitle}>3. Combined Data</h2>
            <p className={styles.combinedDataText}>{combinedData}</p>
          </div>
        </div>

        {/* Arrow pointing down */}
        <div className={styles.arrowContainer}>
          <svg xmlns="http://www.w3.org/2000/svg" className={styles.iconGray} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
          </svg>
        </div>

        {/* Hash Output Block */}
        <div className={`${styles.hashContainer} ${isFound ? styles.hashContainerSuccess : styles.hashContainerError}`}>
          <div className={styles.hashContent}>
            <div className={styles.hashText}>
              <h2 className={styles.blockTitle}>4. Resulting Hash (SHA-256)</h2>
              <p className={styles.hashValue}>{renderHash()}</p>
            </div>
            <div className={styles.hashIcon}>
              {isFound ? <CheckIcon /> : <XCircleIcon />}
            </div>
          </div>
        </div>

        {/* Mining Controls */}
        <div className={styles.buttonContainer}>
          {!isMining ? (
            <button onClick={handleMineClick} className={`${styles.button} ${styles.buttonCyan}`}>
              Auto-Mine
            </button>
          ) : (
            <button onClick={handleStopClick} className={`${styles.button} ${styles.buttonYellow}`}>
              Stop Mining
            </button>
          )}
          <button onClick={handleNewChallengeClick} className={`${styles.button} ${styles.buttonIndigo}`}>
            New Challenge
          </button>
          <button onClick={handleResetClick} className={`${styles.button} ${styles.buttonGray}`}>
            Reset Nonce
          </button>
        </div>
      </div>
    </div>
  );
}
