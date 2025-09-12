import { render, h, Fragment } from "preact";
import { useState, useEffect } from "preact/hooks";
import { g, j, r, u, x } from "./xeact.js";
import { Sha256 } from "@aws-crypto/sha256-js";

/** @jsx h */
/** @jsxFrag Fragment */

function toHexString(arr: Uint8Array) {
  return Array.from(arr)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");
}

interface PreactInfo {
  redir: string;
  challenge: string;
  difficulty: number;
  connection_security_message: string;
  loading_message: string;
  pensive_url: string;
}

const App = () => {
  const [state, setState] = useState<PreactInfo>();
  const [imageURL, setImageURL] = useState<string | null>(null);
  const [passed, setPassed] = useState<boolean>(false);
  const [challenge, setChallenge] = useState<string | null>(null);

  useEffect(() => {
    setState(j("preact_info"));
  });

  useEffect(() => {
    if (state === undefined) {
      return;
    }

    setImageURL(state?.pensive_url);
    const hash = new Sha256("");
    hash.update(state.challenge);
    setChallenge(toHexString(hash.digestSync()));
  }, [state]);

  useEffect(() => {
    if (state === undefined) {
      return;
    }

    const timer = setTimeout(() => {
      setPassed(true);
    }, state?.difficulty * 125);

    return () => clearTimeout(timer);
  }, [challenge]);

  useEffect(() => {
    if (state === undefined) {
      return;
    }

    if (challenge === null) {
      return;
    }

    window.location.href = u(state.redir, {
      result: challenge,
    });
  }, [passed]);

  return (
    <>
      {imageURL !== null && (
        <img src={imageURL} style={{ width: "100%", maxWidth: "256px" }} />
      )}
      {state !== undefined && (
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
