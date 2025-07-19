export function sortMapByKeyAsc<K, V>(map: Map<K, V>): Map<K, V> {
  return new Map(
    Array.from(map.entries()).sort(([keyA], [keyB]) => {
      if (keyA > keyB) return 1;
      if (keyA < keyB) return -1;
      return 0;
    }),
  );
}
