import type { ReactNode } from "react";
import clsx from "clsx";
import Heading from "@theme/Heading";
import styles from "./styles.module.css";

type FeatureItem = {
  title: string;
  imageURL: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: "Easy to Use",
    imageURL: require("@site/static/img/anubis/happy.webp").default,
    description: (
      <>
        Anubis sits in the background and weighs the risk of incoming requests.
        If it asks a client to complete a challenge, no user interaction is
        required.
      </>
    ),
  },
  {
    title: "Lightweight",
    imageURL: require("@site/static/img/anubis/pensive.webp").default,
    description: (
      <>
        Anubis is so lightweight you'll forget it's there until you look at your
        hosting bill. On average it uses less than 128 MB of ram.
      </>
    ),
  },
  {
    title: "Block the scrapers",
    imageURL: require("@site/static/img/anubis/reject.webp").default,
    description: (
      <>
        Anubis uses a combination of heuristics to identify and block bots
        before they take your website down. You can customize the rules with{" "}
        <a href="/docs/admin/policies">your own policies</a>.
      </>
    ),
  },
];

function Feature({ title, description, imageURL }: FeatureItem) {
  return (
    <div className={clsx("col col--4")}>
      <div className="text--center">
        <img src={imageURL} className={styles.featureSvg} role="img" />
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
