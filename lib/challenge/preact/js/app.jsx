import { render, h, Fragment } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { g, j, u, x } from "./xeact.js";
import { Sha256 } from '@aws-crypto/sha256-js';

/** @jsx h */
/** @jsxFrag Fragment */

function toHexString(arr) {
  return Array.from(arr)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");
}

const App = () => {
  const [state, setState] = useState(null);
  const [imageURL, setImageURL] = useState(null);
  const [passed, setPassed] = useState(false);
  const [challenge, setChallenge] = useState(null);

  useEffect(() => {
    setState(j("preact_info"));
  });

  useEffect(() => {
    setImageURL(state.pensive_url);
    const hash = new Sha256('');
    hash.update(state.challenge);
    setChallenge(toHexString(hash.digestSync()));
  }, [state]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setPassed(true);
    }, state.difficulty * 100);

    return () => clearTimeout(timer);
  }, [challenge]);

  useEffect(() => {
    window.location.href = u(state.redir, {
      result: challenge,
    });
  }, [passed]);

  return (
    <>
      {imageURL !== null && (
        <img src={imageURL} style="width:100%;max-width:256px;" />
      )}
      {state !== null && (
        <>
          <p id="status">{state.loading_message}</p>
          <p>{state.connection_security_message}</p>
        </>
      )}
    </>
  );
};

x(g("app"));
render(<App />, g("app"));