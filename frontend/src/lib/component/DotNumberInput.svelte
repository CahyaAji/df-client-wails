<script lang="ts">
  export let value: number = 0;
  export let disabled: boolean = false;
  export let readonly: boolean = false;
  export let placeholder: string = "";

  function normalizeToDot(input: string): string {
    let result = input.replace(/,/g, ".");
    result = result.replace(/[^0-9.\-]/g, "").replace(/(\..*)\./g, "$1");
    return result;
  }

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    const normalized = normalizeToDot(target.value);

    if (target.value !== normalized) {
      target.value = normalized;
    }

    const num = parseFloat(normalized);
    value = isNaN(num) ? 0 : num;
  }

  function handleBlur() {
    if (!isNaN(value)) {
      value = Number(value.toFixed(3));
    }
  }
</script>

<input
  class="input-freq"
  type="text"
  inputmode="decimal"
  {disabled}
  {readonly}
  {placeholder}
  value={isNaN(value) ? "" : value.toString()}
  on:input={handleInput}
  on:blur={handleBlur}
/>

<style>
  .input-freq {
    padding: 4px 8px;
    width: 90px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .input-freq:disabled {
    background-color: white;
    color: black;
  }
</style>
