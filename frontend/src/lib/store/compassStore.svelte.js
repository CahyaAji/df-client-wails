import { readCompass } from "../utils/api_handler";

class CompassStore {
  data = $state(/** @type {number} */ (0));
  error = $state(/** @type {string | null} */ (null));
  isLoading = $state(false);

  /** @type {ReturnType<typeof setInterval> | null} */
  #interval = null;

  get isRunning() {
    return this.#interval !== null;
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

  start() {
    if (this.#interval) {
      return;
    }

    this.fetch();
    this.#interval = setInterval(() => {
      this.fetch();
    }, 1000);

    console.log("Compass store started");
  }

  stop() {
    if (this.#interval) {
      clearInterval(this.#interval);
      this.#interval = null;
      console.log("Compass store stopped");
    }
  }
  clear() {
    this.data = 0;
    this.error = null;
    this.isLoading = false;
  }
}

export const compassStore = new CompassStore();
