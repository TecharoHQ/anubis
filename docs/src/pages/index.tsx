import type { ReactNode } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import HomepageFeatures from "@site/src/components/HomepageFeatures";
import Heading from "@theme/Heading";

import styles from "./index.module.css";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero hero--primary", styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link className="button button--secondary button--lg" to="/docs/">
            Get started
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title={`Anubis: self hostable scraper defense software`}
      description="Weigh the soul of incoming HTTP requests using proof-of-work to stop AI crawlers"
    >
      <HomepageHeader />
      <main>
        <HomepageFeatures />

        <center>
          <p>
            This is all placeholder text. It will be fixed. Give me time. I am
            one person and my project has unexpectedly gone viral.
          </p>
        </center>
      </main>
    </Layout>
  );
}
