// Translations. The app's source strings are English (the default); this file holds the Italian ones.
// Language and currency are app-wide settings stored on the server (see /api/settings).

const IT = {
  // navigation
  'Home': 'Home',
  'Transactions': 'Movimenti',
  'Summary': 'Riepilogo',
  'Accounts': 'Conti',
  'Categories': 'Categorie',
  'Recurring': 'Ricorrenti',
  'Settings': 'Impostazioni',
  'More': 'Altro',
  'Previous': 'Precedente',
  'Next': 'Successivo',
  'Loading…': 'Caricamento…',
  'Could not reach the server.': 'Impossibile contattare il server.',
  'Sign-in required': 'Accesso richiesto',
  'Error': 'Errore',

  // common
  'Cancel': 'Annulla',
  'Save': 'Salva',
  'Delete': 'Elimina',
  'Edit': 'Modifica',
  'Name': 'Nome',
  'Note': 'Nota',
  'Date': 'Data',
  'Type': 'Tipo',
  'Active': 'Attiva',
  'New': 'Nuova',
  'No data': 'Nessun dato',

  // kinds, frequencies, account types
  'Expense': 'Uscita',
  'Income': 'Entrata',
  'Transfer': 'Trasferimento',
  'Every week': 'Ogni settimana',
  'Every month': 'Ogni mese',
  'Every year': 'Ogni anno',
  'Checking': 'Conto corrente',
  'Cash': 'Contanti',
  'Savings': 'Risparmio',
  'Card': 'Carta',
  'Investments': 'Investimenti',

  // summary figures
  'Earned': 'Guadagnato',
  'Spent': 'Speso',
  'Saved': 'Risparmiato',
  'Hide balances': 'Nascondi i saldi',
  'Show balances': 'Mostra i saldi',

  // transaction modal
  'New transaction': 'Nuovo movimento',
  'Edit transaction': 'Modifica movimento',
  'New recurring': 'Nuova ricorrenza',
  'Edit recurring': 'Modifica ricorrenza',
  'Amount ({symbol})': 'Importo ({symbol})',
  'Date of the first one': 'Prima data',
  'Account': 'Conto',
  'From account': 'Da conto',
  'To account': 'A conto',
  'Category': 'Categoria',
  'No category': 'Senza categoria',
  'Frequency': 'Frequenza',
  'End (optional)': 'Fine (opzionale)',
  'Enter an amount greater than zero': 'Inserisci un importo maggiore di zero',
  'Delete the recurring item?': 'Eliminare la ricorrenza?',
  'Delete the transaction?': 'Eliminare il movimento?',
  'Transactions already created are kept.': 'I movimenti già generati restano.',
  'This cannot be undone.': 'L\'operazione non si può annullare.',
  'No account': 'Nessun conto',
  'Create an account first from the Accounts section.': 'Crea prima un conto dalla sezione Conti.',
  'Go to accounts': 'Vai ai conti',
  'recurring': 'ricorrente',
  'No transactions': 'Nessun movimento',

  // recurring choice
  'Edit recurring transaction': 'Modifica movimento ricorrente',
  'Delete recurring transaction': 'Elimina movimento ricorrente',
  'This transaction is part of a recurring series. What do you want to edit?': 'Questo movimento fa parte di una ricorrenza. Cosa vuoi modificare?',
  'This transaction is part of a recurring series. What do you want to delete?': 'Questo movimento fa parte di una ricorrenza. Cosa vuoi eliminare?',
  'Only this transaction': 'Solo questo movimento',
  'This and the following': 'Questo e i successivi',
  'All transactions in the series': 'Tutti i movimenti della ricorrenza',
  'The series stops.': 'La ricorrenza si ferma.',
  'The series will use the new values.': 'La ricorrenza userà i nuovi valori.',
  'Also deletes the series.': 'Elimina anche la ricorrenza.',
  'Past ones too.': 'Anche quelli passati.',
  'The date changes only for this transaction.': 'La data cambia solo per questo movimento.',

  // home, lists
  'Last transactions': 'Ultimi movimenti',
  'Spending by category': 'Spese per categoria',
  'Income by category': 'Entrate per categoria',
  'No accounts yet.': 'Nessun conto.',
  'Create one': 'Creane uno',
  'Month': 'Mese',
  'Year': 'Anno',
  'All accounts': 'Tutti i conti',
  'All categories': 'Tutte le categorie',
  'All types': 'Tutti i tipi',
  'Show more': 'Mostra altri',
  'Income and spending by month': 'Entrate e spese per mese',
  'Trend {year}': 'Andamento {year}',
  'income': 'entrate',
  'spending': 'spese',

  // accounts
  'New account': 'Nuovo conto',
  'Edit account': 'Modifica conto',
  'Opening balance ({symbol})': 'Saldo iniziale ({symbol})',
  'Archived (hidden from new transactions)': 'Archiviato (nascosto dai nuovi movimenti)',
  'Invalid opening balance': 'Saldo iniziale non valido',
  'Delete “{name}”?': 'Eliminare «{name}»?',
  'Only an account without transactions can be deleted; otherwise archive it.': 'Si può eliminare solo un conto senza movimenti; altrimenti archivialo.',
  'Total': 'Totale',
  'Create your first account': 'Crea il tuo primo conto',
  'Archived': 'Archiviati',
  'Group (optional)': 'Gruppo (facoltativo)',
  'Add account to “{name}”': 'Aggiungi un conto a «{name}»',
  'e.g. a bank or a broker': 'es. una banca o un broker',
  'Value updated by hand (e.g. investments)': 'Valore aggiornato a mano (es. investimenti)',
  'Do not use for expenses': 'Non usare per le uscite',
  'The account is not offered for new expenses. Income and transfers still are.': 'Il conto non viene proposto per le nuove uscite. Entrate e giroconti sì.',
  'Delete only the rule': 'Elimina solo la ricorrenza',
  'Delete the rule and its transactions': 'Elimina la ricorrenza e i suoi movimenti',
  'All transactions created by this rule are deleted.': 'Tutti i movimenti generati da questa ricorrenza vengono eliminati.',
  'Change only the next ones': 'Cambia solo i prossimi',
  'Transactions already created stay as they are.': 'I movimenti già generati restano come sono.',
  'Change the transactions already created too': 'Cambia anche i movimenti già generati',
  'Amount, accounts, category and note; dates stay.': 'Importo, conti, categoria e nota; le date restano.',
  'Transfers made after the last update are added to the value.': 'I giroconti fatti dopo l’ultimo aggiornamento si sommano al valore.',
  'Update value': 'Aggiorna valore',
  'Update value: {name}': 'Aggiorna valore: {name}',
  'Value on': 'Valore al',
  'Value ({symbol})': 'Valore ({symbol})',
  'Invalid value': 'Valore non valido',
  'History': 'Storico',
  'No values yet.': 'Nessun valore registrato.',
  'gain/loss': 'guadagno/perdita',
  'value of {date}': 'valore del {date}',
  'value not set yet': 'valore non ancora impostato',
  '1 account': '1 conto',
  '{n} accounts': '{n} conti',
  'Delete this value?': 'Eliminare questo valore?',
  'The balance goes back to the previous value.': 'Il saldo torna al valore precedente.',

  // categories
  'Rename': 'Rinomina',
  'New subcategory of {name}': 'Nuova sottocategoria di {name}',
  'New expense category': 'Nuova categoria di spesa',
  'New income category': 'Nuova categoria di entrata',
  'New category': 'Nuova categoria',
  'Expenses': 'Spese',
  'Income|plural': 'Entrate',
  'No expense categories': 'Nessuna categoria di spesa',
  'No income categories': 'Nessuna categoria di entrata',
  '+ Sub': '+ Sotto',
  'Its subcategories will be deleted too.': 'Verranno eliminate anche le sue sottocategorie.',
  'There are transactions or recurring items in this category. Choose where to move them.': 'Ci sono movimenti o ricorrenze in questa categoria. Scegli dove spostarli.',
  'Move to': 'Sposta in',
  'Create another category of the same type first.': 'Crea prima un\'altra categoria dello stesso tipo.',
  'Move and delete': 'Sposta ed elimina',

  // recurring page
  'paused': 'in pausa',
  'next {date}': 'prossima {date}',
  'stopped': 'ferma',
  'Pause': 'Pausa',
  'Resume': 'Riprendi',
  'No recurring items. Add rent, salary, subscriptions…': 'Nessuna ricorrenza. Aggiungi affitto, stipendio, abbonamenti…',

  // pro
  'Pro': 'Pro',
  'Gross': 'Lordo',
  'Net': 'Netto',
  'By category': 'Per categoria',
  'Payments': 'Versamenti',
  'Months': 'Mesi',
  'No payments': 'Nessun versamento',
  'Setup': 'Impostazioni',
  'Clients': 'Clienti',
  'Client': 'Cliente',
  'Deduction categories': 'Categorie di trattenute',
  'No clients yet': 'Nessun cliente',
  'No categories yet. Add the ones you need: tax, contributions…': 'Nessuna categoria. Aggiungi quelle che ti servono: tasse, contributi…',
  'New payment': 'Nuovo versamento',
  'Edit payment': 'Modifica versamento',
  'Delete the payment?': 'Eliminare il versamento?',
  'New client': 'Nuovo cliente',
  'Edit client': 'Modifica cliente',
  'Edit category': 'Modifica categoria',
  'Payments already registered keep it.': 'I versamenti già registrati lo mantengono.',
  'Gross amount ({symbol})': 'Importo lordo ({symbol})',
  'Split of the amount': 'Ripartizione dell’importo',
  '+ Add a line': '+ Aggiungi una riga',
  'Remove': 'Rimuovi',
  'Choose a client': 'Scegli un cliente',
  'Create a client first.': 'Crea prima un cliente.',
  'Go to setup': 'Vai alle impostazioni',
  'Create categories (tax, contributions…) in the setup to split the amount.': 'Crea le categorie (tasse, contributi…) nelle impostazioni per ripartire l’importo.',
  'The split is higher than the gross amount.': 'La ripartizione supera l’importo lordo.',

  // settings
  'Quick unlock': 'Sblocco rapido',
  'After signing in with the password you can unlock with a PIN. The app locks itself after 5 minutes in the background.': 'Dopo il primo accesso con la password puoi rientrare con un PIN. L\'app si blocca da sola dopo 5 minuti in background.',
  'After signing in with the password you can unlock with a PIN or biometrics. The app locks itself after 5 minutes in the background.': 'Dopo il primo accesso con la password puoi rientrare con un PIN o con la biometria. L\'app si blocca da sola dopo 5 minuti in background.',
  'Current password': 'Password attuale',
  'PIN (4-6 digits)': 'PIN (4-6 cifre)',
  'New PIN (4-6 digits)': 'Nuovo PIN (4-6 cifre)',
  'Turn on PIN': 'Attiva PIN',
  'Change PIN': 'Cambia PIN',
  'Turn off PIN': 'Disattiva PIN',
  'PIN turned on': 'PIN attivato',
  'PIN updated': 'PIN aggiornato',
  'PIN turned off': 'PIN disattivato',
  'Biometrics are on for this device.': 'Biometria attiva su questo dispositivo.',
  'Turn off biometrics': 'Disattiva biometria',
  'Biometrics turned off': 'Biometria disattivata',
  'Turn on fingerprint / Face ID on this device': 'Attiva impronta / Face ID su questo dispositivo',
  'Biometrics turned on': 'Biometria attivata',
  'Biometrics are not available on this device.': 'La biometria non è disponibile su questo dispositivo.',
  'Biometrics need HTTPS: open the app from an https:// address (reverse proxy, Tailscale, domain).': 'La biometria richiede HTTPS: apri l\'app da un indirizzo https:// (reverse proxy, Tailscale, dominio).',
  'Operation failed': 'Operazione non riuscita',
  'Lock now': 'Blocca ora',
  'Appearance': 'Aspetto',
  'Theme': 'Tema',
  'Automatic (system)': 'Automatico (sistema)',
  'Light': 'Chiaro',
  'Dark': 'Scuro',
  'Midnight': 'Mezzanotte',
  'Sand': 'Sabbia',
  'Saved on this device only.': 'Salvato solo su questo dispositivo.',
  'Language and currency': 'Lingua e valuta',
  'Language': 'Lingua',
  'Currency': 'Valuta',
  'Changing the currency only changes the symbol: amounts are not converted.': 'Cambiare la valuta cambia solo il simbolo: gli importi non vengono convertiti.',
  'Euro': 'Euro',
  'US dollar': 'Dollaro statunitense',
  'Australian dollar': 'Dollaro australiano',
  'New Zealand dollar': 'Dollaro neozelandese',
  'British pound': 'Sterlina britannica',
  'Change password': 'Cambia password',
  'New password (min. 6 characters)': 'Nuova password (min. 6 caratteri)',
  'Update': 'Aggiorna',
  'Password updated. PIN and biometrics must be set up again.': 'Password aggiornata. PIN e biometria vanno riattivati.',
  'Data': 'Dati',
  'The CSV has the columns: date, type, amount, account, destination_account, category, subcategory, note. Missing accounts and categories are created.': 'Il CSV ha le colonne: data, tipo, importo, conto, conto_destinazione, categoria, sottocategoria, nota. Conti e categorie mancanti vengono creati.',
  'Export CSV': 'Esporta CSV',
  'Import CSV': 'Importa CSV',
  'Download backup': 'Scarica backup',
  'Imported {n} transactions': 'Importati {n} movimenti',
  'Sign out': 'Esci',
  'Version {version}': 'Versione {version}',

  // sign-in and lock screen
  'Choose a password to protect your data.': 'Scegli una password per proteggere i tuoi dati.',
  'Enter the password.': 'Inserisci la password.',
  'Password': 'Password',
  'Repeat the password': 'Ripeti la password',
  'The passwords do not match': 'Le password non coincidono',
  'Get started': 'Inizia',
  'Sign in': 'Entra',
  '← Back to PIN': '← Torna al PIN',
  'Enter your PIN': 'Inserisci il PIN',
  'Unlock with biometrics': 'Sblocca con la biometria',
  'Biometric unlock failed': 'Biometria non riuscita',
  'Use biometrics': 'Usa la biometria',
  'Backspace': 'Cancella',
  'Unlock': 'Sblocca',
  'Use the password': 'Usa la password',

  // messages sent by the server
  'not found': 'non trovato',
  'name is required': 'il nome è obbligatorio',
  'choose a client': 'scegli un cliente',
  'a category is used twice in the same payment': 'una categoria è usata due volte nello stesso versamento',
  'internal error': 'errore interno',
  'invalid request': 'richiesta non valida',
  'invalid id': 'id non valido',
  'sign-in required': 'accesso richiesto',
  'file too large': 'file troppo grande',
  'unsupported language': 'lingua non supportata',
  'unsupported currency': 'valuta non supportata',
  'account name is required': 'il nome del conto è obbligatorio',
  'invalid account type': 'tipo di conto non valido',
  'this account does not use manual values': 'questo conto non usa valori manuali',
  'the account has transactions or recurring items: archive it instead of deleting it': 'il conto ha movimenti o ricorrenze: archivialo invece di eliminarlo',
  'category name is required': 'il nome della categoria è obbligatorio',
  'parent category does not exist': 'categoria principale inesistente',
  'only two levels are allowed (category and subcategory)': 'sono ammessi solo due livelli (categoria e sottocategoria)',
  'invalid category type': 'tipo categoria non valido',
  'the category has transactions: choose where to move them': 'la categoria ha movimenti: scegli dove spostarli',
  'the destination category cannot be the one being deleted': 'la categoria di destinazione non può essere quella eliminata',
  'destination category does not exist': 'categoria di destinazione inesistente',
  'the destination category must be of the same type': 'la categoria di destinazione deve essere dello stesso tipo',
  'amount must be greater than zero': 'l\'importo deve essere maggiore di zero',
  'account does not exist': 'conto inesistente',
  'choose the destination account': 'scegli il conto di destinazione',
  'the destination account must differ from the source account': 'il conto di destinazione deve essere diverso da quello di origine',
  'destination account does not exist': 'conto di destinazione inesistente',
  'category does not exist': 'categoria inesistente',
  'the category type does not match (income/expense)': 'la categoria non è del tipo giusto (entrata/uscita)',
  'invalid transaction type': 'tipo di movimento non valido',
  'invalid frequency': 'frequenza non valida',
  'the end date is before the start date': 'la data di fine è precedente all\'inizio',
  'invalid period': 'periodo non valido',
  'invalid scope': 'ambito non valido',
  'the CSV has no rows': 'il CSV non contiene righe',
  'the password must be at least 6 characters': 'la password deve avere almeno 6 caratteri',
  'password already set': 'password già impostata',
  'password not set yet': 'password non ancora impostata',
  'wrong password': 'password errata',
  'current password is wrong': 'password attuale errata',
  'the PIN must have 4 to 6 digits': 'il PIN deve avere da 4 a 6 cifre',
  'use the password': 'usa la password',
  'too many attempts: sign in with the password': 'troppi tentativi: accedi con la password',
  'wrong PIN': 'PIN errato',
  'invalid biometric data': 'dati biometrici non validi',
  'biometric request is not valid for this address': 'richiesta biometrica non valida per questo indirizzo',
  'biometric request expired: try again': 'richiesta biometrica scaduta: riprova',
  'domain mismatch': 'dominio non corrispondente',
  'biometric verification was not performed': 'verifica biometrica non eseguita',
  'biometric unlock requires HTTPS': 'lo sblocco biometrico richiede HTTPS',
  'device not recognised: sign out and sign in with the password': 'dispositivo non riconosciuto: esci e rientra con la password',
  'device not recognised': 'dispositivo non riconosciuto',
  'invalid attestation': 'attestazione non valida',
  'credential already registered': 'credenziale già registrata',
  'biometric unlock is not enabled on this device': 'biometria non attiva su questo dispositivo',
  'biometric unlock failed': 'sblocco biometrico non riuscito',
};

// Messages with parts that change (dates, row numbers…).
const IT_PATTERNS = [
  [/^invalid date: (.*) \(expected YYYY-MM-DD\)$/, (m) => `data non valida: ${m[1]} (atteso AAAA-MM-GG)`],
  [/^invalid amount: (.*)$/, (m) => `importo non valido: ${m[1]}`],
  [/^invalid CSV: (.*)$/, (m) => `CSV non valido: ${m[1]}`],
  [/^missing column (.*)$/, (m) => `manca la colonna ${m[1]}`],
  [/^row (\d+): unknown type (.*)$/, (m) => `riga ${m[1]}: tipo sconosciuto ${m[2]}`],
  [/^row (\d+): missing account$/, (m) => `riga ${m[1]}: conto mancante`],
  [/^row (\d+): transfer without a destination account$/, (m) => `riga ${m[1]}: trasferimento senza conto di destinazione`],
  [/^row (\d+): (.*)$/, (m, tr) => `riga ${m[1]}: ${tr(m[2])}`],
];

const CURRENCIES = { EUR: 'Euro', USD: 'US dollar', AUD: 'Australian dollar', NZD: 'New Zealand dollar', GBP: 'British pound' };
const LOCALES = { en: 'en-GB', it: 'it-IT' };

let lang = 'en';
let currency = 'EUR';
// USD shows a plain "$" in every language (it-IT would print "USD"); AUD and NZD keep "A$" / "NZ$" so they stay distinct.
const makeFormat = (lng, cur) => new Intl.NumberFormat(LOCALES[lng], { style: 'currency', currency: cur, currencyDisplay: cur === 'USD' ? 'narrowSymbol' : 'symbol' });
let moneyFmt = makeFormat('en', currency);

function fill(s, vars) {
  return vars ? s.replace(/\{(\w+)\}/g, (_, k) => (vars[k] === undefined ? '' : vars[k])) : s;
}

/**
 * Translate a UI string. The English text is the key; an optional "|context" suffix tells apart
 * identical English strings that need different translations, and is never shown.
 */
export function t(key, vars) {
  return fill((lang === 'it' && IT[key]) || key.replace(/\|.*$/, ''), vars);
}

/** Translate a message coming from the server. */
export function tr(msg) {
  if (lang !== 'it' || !msg) return msg;
  if (IT[msg]) return IT[msg];
  for (const [re, f] of IT_PATTERNS) {
    const m = msg.match(re);
    if (m) return f(m, tr);
  }
  return msg;
}

export function configure(newLang, newCurrency) {
  lang = LOCALES[newLang] ? newLang : 'en';
  currency = CURRENCIES[newCurrency] ? newCurrency : 'EUR';
  moneyFmt = makeFormat(lang, currency);
  document.documentElement.lang = lang;
}

export const locale = () => LOCALES[lang];
export const language = () => lang;
export const currencies = () => Object.entries(CURRENCIES);
export const money = (cents) => moneyFmt.format((cents || 0) / 100);
export const currencySymbol = () => moneyFmt.formatToParts(0).find((p) => p.type === 'currency')?.value || currency;

/** Format cents for an amount field, using the language's decimal separator. */
export const toInput = (cents) => {
  const s = (cents / 100).toFixed(2);
  return lang === 'it' ? s.replace('.', ',') : s;
};
export const amountPlaceholder = () => toInput(0);

/** Parse what the user typed ("1.234,56", "1,234.56", "12,5", "12.5") into cents; NaN if invalid. */
export function parseCents(input) {
  let s = String(input).replace(/[^\d.,-]/g, '');
  const comma = s.lastIndexOf(','), dot = s.lastIndexOf('.');
  if (comma >= 0 && dot >= 0) {
    // the separator that comes last is the decimal one
    s = comma > dot ? s.replace(/\./g, '').replace(',', '.') : s.replace(/,/g, '');
  } else if (comma >= 0) {
    s = s.replace(',', '.');
  }
  const n = parseFloat(s);
  return Number.isFinite(n) ? Math.round(n * 100) : NaN;
}
