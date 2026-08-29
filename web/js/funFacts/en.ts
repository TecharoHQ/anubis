// English fun facts displayed while users wait for proof-of-work verification.
//
// To add a new language:
//   1. Create <lang>.ts in this directory exporting `funFacts: string[]`.
//   2. Import it in index.ts and add it to the `funFactsByLang` map.
//   3. Run `npm run assets` — the build emits static/funfacts/<lang>.json.
export const funFacts: string[] = [
  "Did you know? The shortest complete sentence in English is 'Go.'",
  "Fun fact: Honey never spoils. Archaeologists have found edible honey in ancient Egyptian tombs.",
  "Interesting: A day on Venus is longer than a year on Venus.",
  "Did you know? Octopuses have three hearts and blue blood.",
  "Fun fact: The Eiffel Tower can grow up to 15cm taller during hot summer days.",
  "Interesting: A group of flamingos is called a 'flamboyance'.",
  "Did you know? Bananas are berries, but strawberries aren't.",
  "Fun fact: There are more stars in the universe than grains of sand on Earth.",
  "Interesting: The human body has enough iron to make a 3-inch nail.",
  "Did you know? A snail can sleep for three years.",
  "Fun fact: A single cloud can weigh over a million pounds.",
  "Interesting: The invention of the wheel predates the invention of the wheelbarrow by thousands of years.",
  "Did you know? The longest place name on Earth has 85 letters: Taumatawhakatangihangakoauauotamateaturipukakapikimaungahoronukupokaiwhenuakitanatahu.",
  "Fun fact: Hot water freezes faster than cold water under certain conditions, known as the Mpemba effect.",
  "Interesting: The Atlantic Ocean is saltier than the Pacific Ocean.",
  "Did you know? A day on Mars is about 24 hours and 40 minutes long.",
  "Fun fact: The first computer mouse was made of wood.",
  "Interesting: The Earth's rotation is gradually slowing down, lengthening our days by about 1.7 milliseconds per century.",
  "Did you know? The human nose can remember 50,000 different scents.",
  "Fun fact: The first website ever created is still online at info.cern.ch.",
  "Interesting: The shortest war in history lasted only 38 minutes between Britain and Zanzibar in 1896.",
];
