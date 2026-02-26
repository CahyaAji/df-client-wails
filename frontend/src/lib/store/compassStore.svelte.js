import { readCompass } from "../utils/api_handler";

class CompassStore {
  data = $state(/** @type {number} */ (0));
  error = $state(/** @type {string | null} */ (null));
  isLoading = $state(false);

  /** @type {ReturnType<typeof setTimeout> | null} */
  #timer = null;
  #running = false;

  get isRunning() {
    return this.#running;
  }

  async fetch() {
    this.isLoading = true;
    this.error = null;

    try {
      const result = await readCompass();

      if (result.success) {
        this.data = result.data ?? 0;
        this.isLoading = false;
      } else {
        this.data = 0;
        this.error = result.error ?? null;
        this.isLoading = false;
      }
    } catch (error) {
      this.data = 0;
      this.error = error instanceof Error ? error.message : String(error);
      this.isLoading = false;
      return { success: false, error: error instanceof Error ? error.message : String(error) };
    }
  }

  // Serialized poll: only one fetch in-flight at a time.
  async #poll() {
    if (!this.#running) return;
    await this.fetch();
    if (this.#running) {
      this.#timer = setTimeout(() => this.#poll(), 1000);
    }
  }

  start() {
    if (this.#running) return;
    this.#running = true;
    this.#timer = setTimeout(() => this.#poll(), 100);
    console.log("Compass store started");
  }

  stop() {
    this.#running = false;
    if (this.#timer) {
      clearTimeout(this.#timer);
      this.#timer = null;
    }
    console.log("Compass store stopped");
  }
}

export const compassStore = new CompassStore();
