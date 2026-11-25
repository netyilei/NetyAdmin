import { defineStore } from "pinia";
import { ref } from "vue";

export const useKeepAliveStore = defineStore("geeker-keepAlive", () => {
  const keepAliveName = ref<string[]>([]);

  // Add KeepAliveName
  const addKeepAliveName = (name: string) => {
    if (!keepAliveName.value.includes(name)) {
      keepAliveName.value.push(name);
    }
  };

  // Remove KeepAliveName
  const removeKeepAliveName = (name: string) => {
    keepAliveName.value = keepAliveName.value.filter(item => item !== name);
  };

  // Set KeepAliveName
  const setKeepAliveName = (newKeepAliveName: string[] = []) => {
    keepAliveName.value = newKeepAliveName;
  };

  return {
    keepAliveName,
    addKeepAliveName,
    removeKeepAliveName,
    setKeepAliveName
  };
});
