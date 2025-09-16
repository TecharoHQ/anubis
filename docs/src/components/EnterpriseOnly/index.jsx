import styles from './styles.module.css';

export default function EnterpriseOnly({ link }) {
  return (
    <a className={styles.link} href={link}>
      <div className={styles.container}>
        <span className={styles.label}>BotStopper Only</span>
      </div>
    </a>
  );
}