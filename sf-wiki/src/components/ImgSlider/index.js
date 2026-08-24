import { useState } from 'react';

import styles from './styles.module.css';

/**
 * A screenshot slider, ported from ~/cwcloud-website's src/components/ImgSlider.
 *
 * Same props as there, so a page can be written the same way in either wiki:
 *
 *   import ImgSlider from '@site/src/components/ImgSlider';
 *
 *   export const images = [
 *     { filename: "signup-coach.png", label: "Signing up as a coach" },
 *     { filename: "sidebar.png", label: "The sidebar" },
 *   ]
 *
 *   <ImgSlider items={images} path="/img/screenshots" print_label />
 *
 * `path` is a public URL under static/, not a relative import: these are
 * screenshots served as files, and a tutorial that walks through six of them
 * should not make the reader scroll past six full-width images to reach the
 * next paragraph.
 *
 * The file must be .mdx for the import to work - Docusaurus 3 reads .md as
 * plain CommonMark, where JSX is text.
 */
const ImgSlider = ({ items, path, print_label = false }) => {
  const [currentIndex, setCurrentIndex] = useState(0);

  // Nothing to show rather than a crash: an empty list is a page whose
  // screenshots have not been captured yet, which happens while one is
  // being written.
  if (!items || items.length === 0) return null;

  const nextSlide = () => {
    setCurrentIndex((prevIndex) => (prevIndex + 1) % items.length);
  };

  const prevSlide = () => {
    setCurrentIndex((prevIndex) => (prevIndex - 1 + items.length) % items.length);
  };

  const current = items[currentIndex];

  return (
    <div className={styles.slider}>
      <div className={styles.slide}>
        <img
          className={styles.image}
          src={path + '/' + current.filename}
          alt={current.label}
          loading="lazy"
        />
        {print_label ? <p className={styles.label}>{current.label}</p> : ''}

        {/* A single image needs no controls: the arrows would be two buttons
            that do nothing, and the counter would read "1 / 1". */}
        {items.length > 1 ? (
          <div className={styles.controls}>
            <button
              type="button"
              className={styles.button}
              onClick={prevSlide}
              aria-label="Previous screenshot"
            >
              &lt;
            </button>
            <span className={styles.counter}>
              {currentIndex + 1} / {items.length}
            </span>
            <button
              type="button"
              className={styles.button}
              onClick={nextSlide}
              aria-label="Next screenshot"
            >
              &gt;
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
};

export default ImgSlider;
