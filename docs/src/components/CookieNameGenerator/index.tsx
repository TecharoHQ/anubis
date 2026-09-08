import { useEffect, useState } from "react";
import BrowserOnly from "@docusaurus/BrowserOnly";
import Code from "@theme/CodeInline";
import styles from "./styles.module.css";

export type SameSite = "None" | "Lax" | "Strict" | "Default";

// Anubis hashes the SameSite mode as the numeric value of net/http's SameSite
// constants, which start at 1.
const sameSiteValues: Record<SameSite, number> = {
  Default: 1,
  Lax: 2,
  Strict: 3,
  None: 4,
};

const sameSiteOptions: SameSite[] = ["None", "Lax", "Strict", "Default"];

export interface CookieSettings {
  prefix: string;
  httpOnly: boolean;
  secure: boolean;
  partitioned: boolean;
  sameSite: SameSite;
  domain: string;
  dynamicDomain: boolean;
  basePrefix: string;
}

export const defaultSettings: CookieSettings = {
  prefix: "techaro.lol-anubis",
  httpOnly: false,
  secure: true,
  partitioned: true,
  sameSite: "None",
  domain: "",
  dynamicDomain: false,
  basePrefix: "",
};

// settingsFingerprint builds the exact string that Anubis feeds into SHA-256 in
// (*Server).cookieName. Keep this in sync with lib/http.go.
export function settingsFingerprint(settings: CookieSettings): string {
  return [
    settings.httpOnly,
    settings.secure,
    settings.partitioned,
    sameSiteValues[settings.sameSite],
    settings.domain,
    settings.dynamicDomain,
    // Anubis trims trailing slashes off the base prefix before hashing it.
    settings.basePrefix.replace(/\/+$/, ""),
  ].join("|");
}

// settingsSuffix returns the hex-encoded first four bytes of the fingerprint
// hash. This is what Anubis appends to every cookie name it sets.
export async function settingsSuffix(
  settings: CookieSettings,
): Promise<string> {
  const digest = await self.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(settingsFingerprint(settings)),
  );

  return Array.from(new Uint8Array(digest).slice(0, 4))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function Checkbox({
  label,
  envVar,
  checked,
  onChange,
}: {
  label: string;
  envVar: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldName}>
        <Code>{envVar}</Code>
      </span>
      <span className={styles.checkboxRow}>
        <input
          type="checkbox"
          checked={checked}
          onChange={(ev) => onChange(ev.target.checked)}
        />
        <span>{label}</span>
      </span>
    </label>
  );
}

function TextField({
  envVar,
  placeholder,
  value,
  onChange,
}: {
  envVar: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldName}>
        <Code>{envVar}</Code>
      </span>
      <input
        type="text"
        spellCheck={false}
        placeholder={placeholder}
        value={value}
        onChange={(ev) => onChange(ev.target.value)}
      />
    </label>
  );
}

function CookieNameGeneratorImpl() {
  const [settings, setSettings] = useState<CookieSettings>(defaultSettings);
  const [suffix, setSuffix] = useState<string>("");

  const set = <K extends keyof CookieSettings>(
    key: K,
    value: CookieSettings[K],
  ) => setSettings((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    let stale = false;
    settingsSuffix(settings).then((value) => {
      if (!stale) {
        setSuffix(value);
      }
    });
    return () => {
      stale = true;
    };
  }, [settings]);

  const authName = `${settings.prefix}-auth-${suffix}`;
  const testName = `${settings.prefix}-cookie-verification-${suffix}`;

  const downgradesSameSite = settings.sameSite === "None" && !settings.secure;
  const rejectedByBrowsers = settings.partitioned && !settings.secure;

  return (
    <div className={styles.container}>
      <div className={styles.fields}>
        <TextField
          envVar="COOKIE_PREFIX"
          placeholder={defaultSettings.prefix}
          value={settings.prefix}
          onChange={(value) => set("prefix", value)}
        />
        <TextField
          envVar="COOKIE_DOMAIN"
          placeholder="unset"
          value={settings.domain}
          onChange={(value) => set("domain", value)}
        />
        <TextField
          envVar="BASE_PREFIX"
          placeholder="unset"
          value={settings.basePrefix}
          onChange={(value) => set("basePrefix", value)}
        />
        <label className={styles.field}>
          <span className={styles.fieldName}>
            <Code>COOKIE_SAME_SITE</Code>
          </span>
          <select
            value={settings.sameSite}
            onChange={(ev) => set("sameSite", ev.target.value as SameSite)}
          >
            {sameSiteOptions.map((mode) => (
              <option key={mode} value={mode}>
                {mode}
              </option>
            ))}
          </select>
        </label>
        <Checkbox
          envVar="COOKIE_SECURE"
          label="Only send the cookie over HTTPS"
          checked={settings.secure}
          onChange={(value) => set("secure", value)}
        />
        <Checkbox
          envVar="COOKIE_PARTITIONED"
          label="Enable the partitioned (CHIPS) flag"
          checked={settings.partitioned}
          onChange={(value) => set("partitioned", value)}
        />
        <Checkbox
          envVar="COOKIE_HTTP_ONLY"
          label="Hide the cookie from client-side JavaScript"
          checked={settings.httpOnly}
          onChange={(value) => set("httpOnly", value)}
        />
        <Checkbox
          envVar="COOKIE_DYNAMIC_DOMAIN"
          label="Derive the cookie domain from the request host"
          checked={settings.dynamicDomain}
          onChange={(value) => set("dynamicDomain", value)}
        />
      </div>

      <table className={styles.results}>
        <thead>
          <tr>
            <th>Cookie</th>
            <th>Name</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Authorization</td>
            <td>
              <Code>{authName}</Code>
            </td>
          </tr>
          <tr>
            <td>Cookie support test</td>
            <td>
              <Code>{testName}</Code>
            </td>
          </tr>
        </tbody>
      </table>

      <details className={styles.details}>
        <summary>How this name is derived</summary>
        <p>
          Anubis hashes your cookie settings with SHA-256 and appends the first
          four bytes to every cookie name. Changing any setting below changes
          the suffix, so clients holding a cookie from the old settings are
          issued a fresh one instead of failing validation forever.
        </p>
        <p>
          The hashed string for these settings is{" "}
          <Code>{settingsFingerprint(settings)}</Code>, which yields the suffix{" "}
          <Code>{suffix}</Code>.
        </p>
      </details>

      {downgradesSameSite && (
        <div className={styles.note}>
          Anubis serves this cookie as <Code>SameSite=Lax</Code> because{" "}
          <Code>COOKIE_SECURE</Code> is off. The name above is unaffected: the
          hash uses the configured value, not the downgraded one.
        </div>
      )}

      {rejectedByBrowsers && (
        <div className={styles.warning}>
          Browsers reject partitioned cookies that are not also{" "}
          <Code>Secure</Code>. Set <Code>COOKIE_PARTITIONED=false</Code> if you
          need to serve Anubis over plain HTTP.
        </div>
      )}

      <button
        className={styles.reset}
        onClick={() => setSettings(defaultSettings)}
      >
        Reset to defaults
      </button>
    </div>
  );
}

export default function CookieNameGenerator() {
  return (
    <BrowserOnly fallback={<div>Loading...</div>}>
      {() => <CookieNameGeneratorImpl />}
    </BrowserOnly>
  );
}
