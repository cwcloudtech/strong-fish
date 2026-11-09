/// The API's response shapes, decoded into plain Dart objects.
///
/// Every model parses defensively: a field the server hasn't sent (an older
/// deployment, or a payload that legitimately omits it) becomes a sensible
/// default rather than a runtime type error, since a single bad cast takes the
/// whole screen down.
library;

double _toDouble(dynamic value, [double fallback = 0]) {
  if (value is num) return value.toDouble();
  if (value is String) return double.tryParse(value) ?? fallback;
  return fallback;
}

double? _toDoubleOrNull(dynamic value) {
  if (value == null) return null;
  if (value is num) return value.toDouble();
  if (value is String) return double.tryParse(value);
  return null;
}

int _toInt(dynamic value, [int fallback = 0]) {
  if (value is num) return value.toInt();
  if (value is String) return int.tryParse(value) ?? fallback;
  return fallback;
}

int? _toIntOrNull(dynamic value) {
  if (value == null) return null;
  if (value is num) return value.toInt();
  if (value is String) return int.tryParse(value);
  return null;
}

String _toString(dynamic value, [String fallback = '']) => value?.toString() ?? fallback;

Map<String, String> _toLabels(dynamic value) {
  if (value is Map) {
    return value.map((key, label) => MapEntry(key.toString(), label?.toString() ?? ''));
  }
  return const {};
}

DateTime _toDate(dynamic value) => DateTime.tryParse(_toString(value))?.toUtc() ?? DateTime.utc(1970);

/// A date that may legitimately be absent - an event with no stated end -
/// rather than _toDate's epoch fallback, which would render as 1970.
DateTime? _optionalDate(dynamic value) {
  final parsed = DateTime.tryParse(_toString(value));
  return parsed?.toLocal();
}

List<String> _toStringList(dynamic value) {
  if (value is List) return value.map((item) => item.toString()).toList();
  return const [];
}

/// A user as the app knows them: the connected account, or a post's author.
class User {
  final String id;
  final String email;
  final String name;
  final String surname;
  final String handle;
  /// The name the member picked. When [anonymous] is set it is the only name
  /// anybody else sees, and the handle is derived from it.
  final String username;
  final bool anonymous;
  final String bio;
  final String role;
  final String picture;
  final String locale;
  /// Who may read this profile: 'public', 'clubs' or 'private'. An unknown
  /// value reads as the narrowest, matching the API.
  final String profileVisibility;

  /// Whether the member put their fitness data on their public profile for
  /// everyone (off by default).
  final bool fitnessPublic;
  final String birthdate;
  final double bodyweight;

  /// Height in centimetres, 0 when not stated. Nothing is computed from it -
  /// no coefficient uses it - but it is context for whoever reads the
  /// training, the coach and the AI alike.
  final double height;

  /// Which set of strength coefficients applies: 'male' or 'female'.
  /// Powerlifting fits DOTS, Wilks and IPF GL separately, and there is no
  /// unisex variant of any of them. Male is the default.
  final String gender;
  /// The accounts the member chose to show, keyed as the API names them (see
  /// socials.dart). Values are names on the service, never whole URLs.
  final Map<String, String> socials;
  final bool mfaEnabled;
  final String i18nCode;

  const User({
    required this.id,
    this.email = '',
    this.name = '',
    this.surname = '',
    this.handle = '',
    this.username = '',
    this.anonymous = false,
    this.bio = '',
    this.role = '',
    this.picture = '',
    this.locale = '',
    this.profileVisibility = 'private',
    this.fitnessPublic = false,
    this.birthdate = '',
    this.bodyweight = 0,
    this.height = 0,
    this.gender = 'male',
    this.socials = const {},
    this.mfaEnabled = false,
    this.i18nCode = '',
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: _toString(json['id']),
        email: _toString(json['email']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        handle: _toString(json['handle']),
        username: _toString(json['username']),
        anonymous: json['anonymous'] == true,
        bio: _toString(json['bio']),
        role: _toString(json['role']),
        picture: _toString(json['picture']),
        locale: _toString(json['locale']),
        profileVisibility: _toString(json['profileVisibility'], 'private'),
        fitnessPublic: json['fitnessPublic'] == true,
        birthdate: _toString(json['birthdate']),
        bodyweight: _toDouble(json['bodyweight']),
        height: _toDouble(json['height']),
        gender: _toString(json['gender'], 'male'),
        socials: _toLabels(json['socials']),
        mfaEnabled: json['mfaEnabled'] == true,
        i18nCode: _toString(json['i18nCode']),
      );

  String get fullName => '$name $surname'.trim();

  /// Whether the account may create clubs, upload programs and extend the
  /// exercise catalog.
  bool get isCoach => role == 'coach' || role == 'superadmin';
  bool get isSuperadmin => role == 'superadmin';

  /// Whether the account may do anything beyond reading its own status.
  bool get isActive => role != 'disabled' && role != 'ban';
}

/// What a login returned: either a session, or a second-factor challenge.
class LoginResult {
  final String token;
  final bool mfaRequired;
  final String challengeToken;
  final bool hasTotp;
  final bool hasWebAuthn;

  const LoginResult({
    this.token = '',
    this.mfaRequired = false,
    this.challengeToken = '',
    this.hasTotp = false,
    this.hasWebAuthn = false,
  });

  factory LoginResult.fromJson(Map<String, dynamic> json) => LoginResult(
        token: _toString(json['token']),
        mfaRequired: json['mfaRequired'] == true,
        challengeToken: _toString(json['challengeToken']),
        hasTotp: json['hasTotp'] == true,
        hasWebAuthn: json['hasWebAuthn'] == true,
      );
}

/// One member of a club, as the club's roster lists them.
class ClubMember {
  final String userId;
  final String email;
  final String name;
  final String surname;
  final String role;

  const ClubMember({
    required this.userId,
    this.email = '',
    this.name = '',
    this.surname = '',
    this.role = '',
  });

  /// What to call them in a list: their name, falling back to the address,
  /// which is the only other thing every account is guaranteed to have.
  String get label {
    final full = '$name $surname'.trim();
    return full.isEmpty ? email : full;
  }

  factory ClubMember.fromJson(Map<String, dynamic> json) => ClubMember(
        userId: _toString(json['userId']),
        email: _toString(json['email']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        role: _toString(json['role']),
      );
}

class Club {
  final String id;
  final String name;
  final String description;
  final String city;
  final String role;
  final int memberCount;

  const Club({
    required this.id,
    this.name = '',
    this.description = '',
    this.city = '',
    this.role = '',
    this.memberCount = 0,
  });

  factory Club.fromJson(Map<String, dynamic> json) => Club(
        id: _toString(json['id']),
        name: _toString(json['name']),
        description: _toString(json['description']),
        city: _toString(json['city']),
        role: _toString(json['role']),
        memberCount: _toInt(json['memberCount']),
      );

  bool get canManage => role == 'owner' || role == 'admin';
}

class Exercise {
  final String id;
  final String slug;
  final Map<String, String> labels;
  final String category;
  final String oneRmRef;
  final bool bodyweight;
  final bool main;

  const Exercise({
    required this.id,
    this.slug = '',
    this.labels = const {},
    this.category = '',
    this.oneRmRef = '',
    this.bodyweight = false,
    this.main = false,
  });

  factory Exercise.fromJson(Map<String, dynamic> json) => Exercise(
        id: _toString(json['id']),
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        category: _toString(json['category']),
        oneRmRef: _toString(json['oneRmRef']),
        bodyweight: json['bodyweight'] == true,
        main: json['main'] == true,
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// One of the member's maxes. Everything the training screens show is computed
/// from these server-side, so changing one recalculates the whole program.
/// An outside account that counts a member's steps for them, and whether this
/// member has connected it.
///
/// Provider-keyed rather than named after one integration: Google Health is
/// where Fitbit data arrives now that the Fitbit Web API is switched off, and
/// the next one will be a different name again.
class DeviceConnection {
  final String provider;
  final bool available;
  final bool connected;
  final DateTime? connectedAt;

  /// What the provider can be read for: `activity`, `body-composition`, or
  /// both.
  final List<String> capabilities;

  /// Whether this is the one provider whose steps and heart rate are shown.
  /// The server decides, so the phone and the web app cannot disagree when a
  /// member has connected more than one.
  final bool activitySource;

  /// Whether this is the one scale whose body composition is shown - the same
  /// single-source rule, so a member with two scales sees one panel, not two.
  final bool bodyCompositionSource;

  /// Whether this is the one food diary whose nutrition is shown - the same
  /// single-source rule again, so a member with two food apps sees one panel.
  final bool nutritionSource;

  /// Whether this provider can be connected with an email and password, so the
  /// UI shows a sign-in form.
  final bool accountPaired;

  /// Whether this provider offers a consent redirect to connect (Google
  /// Health, Withings), so the UI shows a "connect" button.
  final bool oauth;

  /// The public Google client id for "Sign in with Google", when the provider
  /// supports it (Stance). Empty when unavailable.
  final String googleClientId;

  /// Whether this provider is connected with a single personal API key the
  /// member enters (Garmin), so the UI shows a one-field key form.
  final bool keyPaired;

  const DeviceConnection({
    required this.provider,
    this.available = false,
    this.connected = false,
    this.connectedAt,
    this.capabilities = const [],
    this.activitySource = false,
    this.bodyCompositionSource = false,
    this.nutritionSource = false,
    this.accountPaired = false,
    this.oauth = false,
    this.googleClientId = '',
    this.keyPaired = false,
  });

  bool can(String capability) => capabilities.contains(capability);

  factory DeviceConnection.fromJson(Map<String, dynamic> json) => DeviceConnection(
        provider: _toString(json['provider']),
        available: json['available'] == true,
        connected: json['connected'] == true,
        connectedAt: json['connectedAt'] == null ? null : _toDate(json['connectedAt']),
        capabilities: json['capabilities'] is List
            ? (json['capabilities'] as List).map((item) => item.toString()).toList()
            : const [],
        activitySource: json['activitySource'] == true,
        bodyCompositionSource: json['bodyCompositionSource'] == true,
        nutritionSource: json['nutritionSource'] == true,
        accountPaired: json['accountPaired'] == true,
        oauth: json['oauth'] == true,
        googleClientId: _toString(json['googleClientId']),
        keyPaired: json['keyPaired'] == true,
      );
}

/// One completed barbell-velocity set, as the phone renders it.
class StanceSet {
  final String lift;

  /// Where the set came from - "strongfish" for one we captured ourselves, or
  /// the provider it was read from (e.g. "stance").
  final String source;
  final double weightKg;
  final int reps;
  final double givenRpe;
  final double predictedRpe;
  final double meanVelocity;
  final double peakVelocity;

  /// Each rep's mean concentric velocity in order - the per-set chart.
  final List<double> repVelocities;

  /// Each rep's peak concentric velocity, the faster series drawn over the
  /// mean one - the two lines the bar-speed app itself plots.
  final List<double> repPeakVelocities;

  /// What the set was for - "warmup", "topset" or "backoff". Descriptive only.
  final String setType;

  /// Whether the set was a tempo or paused variation (the bar slowed on
  /// purpose), so a reader does not mistake the slower speed for fatigue.
  final bool tempoVariation;

  const StanceSet({
    this.lift = '',
    this.source = '',
    this.weightKg = 0,
    this.reps = 0,
    this.givenRpe = 0,
    this.predictedRpe = 0,
    this.meanVelocity = 0,
    this.peakVelocity = 0,
    this.repVelocities = const [],
    this.repPeakVelocities = const [],
    this.setType = 'warmup',
    this.tempoVariation = false,
  });

  factory StanceSet.fromJson(Map<String, dynamic> json) => StanceSet(
        lift: _toString(json['lift']),
        source: _toString(json['source']),
        weightKg: _toDouble(json['weightKg']),
        reps: _toInt(json['reps']),
        givenRpe: _toDouble(json['givenRpe']),
        predictedRpe: _toDouble(json['predictedRpe']),
        meanVelocity: _toDouble(json['meanVelocity']),
        peakVelocity: _toDouble(json['peakVelocity']),
        repVelocities: (json['repVelocities'] as List? ?? const [])
            .map((v) => _toDouble(v))
            .toList(),
        repPeakVelocities: (json['repPeakVelocities'] as List? ?? const [])
            .map((v) => _toDouble(v))
            .toList(),
        setType: _toString(json['setType']).isEmpty
            ? 'warmup'
            : _toString(json['setType']),
        tempoVariation: json['tempoVariation'] == true,
      );
}

/// The latest reading of one body composition measure.
class BodyMeasure {
  /// weight, bmi, fatRatio, fatMass, fatFreeMass, muscleMass, hydration,
  /// boneMass, visceralFat or basalMetabolicRate.
  final String type;
  final double value;

  /// kg, %, kcal, kg/m2, or blank for an index.
  final String unit;
  final DateTime? takenAt;

  const BodyMeasure({this.type = '', this.value = 0, this.unit = '', this.takenAt});

  factory BodyMeasure.fromJson(Map<String, dynamic> json) => BodyMeasure(
        type: _toString(json['type']),
        value: _toDouble(json['value']),
        unit: _toString(json['unit']),
        takenAt: json['takenAt'] == null ? null : _toDate(json['takenAt']),
      );
}

/// One day's step count.
class StepDay {
  final String date;
  final int steps;

  const StepDay({this.date = '', this.steps = 0});

  factory StepDay.fromJson(Map<String, dynamic> json) => StepDay(
        date: _toString(json['date']),
        steps: _toInt(json['steps']),
      );
}

/// One day's heart rate, and the hardest zone it reached.
class HeartDay {
  final String date;
  final int avg;
  final int max;
  final int min;

  /// One of light, moderate, vigorous, peak - the provider's own four bands,
  /// which is what the colour reads from.
  final String zone;

  /// Separates "the watch was off" from "a resting day": both would otherwise
  /// be a row of zeroes.
  final bool known;

  const HeartDay({
    this.date = '',
    this.avg = 0,
    this.max = 0,
    this.min = 0,
    this.zone = 'light',
    this.known = false,
  });

  factory HeartDay.fromJson(Map<String, dynamic> json) => HeartDay(
        date: _toString(json['date']),
        avg: _toInt(json['avg']),
        max: _toInt(json['max']),
        min: _toInt(json['min']),
        zone: _toString(json['zone'], 'light'),
        known: json['known'] == true,
      );
}

/// A range of days of heart rate.
class HeartSeries {
  final String provider;
  final String range;
  final List<HeartDay> days;
  final int averageBpm;
  final int peakBpm;

  /// The provider would not reach as far back as was asked - heart rate stops
  /// at a fortnight where steps go back three months. The days are real, they
  /// just cover less than the question did.
  final bool clamped;

  const HeartSeries({
    this.provider = '',
    this.range = '',
    this.days = const [],
    this.averageBpm = 0,
    this.peakBpm = 0,
    this.clamped = false,
  });

  factory HeartSeries.fromJson(Map<String, dynamic> json) => HeartSeries(
        provider: _toString(json['provider']),
        range: _toString(json['range']),
        days: (json['days'] as List? ?? [])
            .map((item) => HeartDay.fromJson(Map<String, dynamic>.from(item as Map)))
            .toList(),
        averageBpm: _toInt(json['averageBpm']),
        peakBpm: _toInt(json['peakBpm']),
        clamped: json['clamped'] == true,
      );
}

/// A range of days and what to say about it.
///
/// The average comes from the API rather than being worked out here, so the
/// phone and the web app cannot disagree about the same week - in particular
/// about what to do with a day the provider had nothing for.
class StepSeries {
  final String provider;
  final String range;
  final String from;
  final String to;
  final List<StepDay> days;
  final int total;
  final int dailyAverage;

  const StepSeries({
    this.provider = '',
    this.range = '',
    this.from = '',
    this.to = '',
    this.days = const [],
    this.total = 0,
    this.dailyAverage = 0,
  });

  factory StepSeries.fromJson(Map<String, dynamic> json) => StepSeries(
        provider: _toString(json['provider']),
        range: _toString(json['range']),
        from: _toString(json['from']),
        to: _toString(json['to']),
        days: (json['days'] as List? ?? [])
            .map((item) => StepDay.fromJson(Map<String, dynamic>.from(item as Map)))
            .toList(),
        total: _toInt(json['total']),
        dailyAverage: _toInt(json['dailyAverage']),
      );
}

/// One day's logged nutrition: the energy eaten and the protein in it.
class NutritionDay {
  final String date;
  final double kcal;
  final double proteinG;

  const NutritionDay({this.date = '', this.kcal = 0, this.proteinG = 0});

  factory NutritionDay.fromJson(Map<String, dynamic> json) => NutritionDay(
        date: _toString(json['date']),
        kcal: _toDouble(json['kcal']),
        proteinG: _toDouble(json['proteinG']),
      );
}

/// The member's ideal daily protein band and maintenance energy, with how each
/// was derived so the UI can warn when a figure is estimated.
class NutritionTargets {
  final double proteinMinG;
  final double proteinMaxG;

  /// The grams-per-kilogram-of-fat-free-mass constants the band came from (1.6
  /// and 2.2), so the UI can print the rule before the kilograms.
  final double proteinPerKgMin;
  final double proteinPerKgMax;
  final double fatFreeMassKg;
  final double maintenanceKcal;
  final double restingKcal;
  final double activityKcal;

  /// True when any input was guessed rather than measured, so the UI shows a
  /// plain "these are approximate" note.
  final bool estimated;

  /// True when nothing could be computed because the height is missing: the UI
  /// turns it into a prompt to fill the profile rather than an empty panel.
  final bool needsHeight;

  const NutritionTargets({
    this.proteinMinG = 0,
    this.proteinMaxG = 0,
    this.proteinPerKgMin = 0,
    this.proteinPerKgMax = 0,
    this.fatFreeMassKg = 0,
    this.maintenanceKcal = 0,
    this.restingKcal = 0,
    this.activityKcal = 0,
    this.estimated = false,
    this.needsHeight = false,
  });

  bool get hasProtein => proteinMaxG > 0;
  bool get hasEnergy => maintenanceKcal > 0;
  bool get isEmpty => !hasProtein && !hasEnergy && !needsHeight;

  factory NutritionTargets.fromJson(Map<String, dynamic> json) => NutritionTargets(
        proteinMinG: _toDouble(json['proteinMinG']),
        proteinMaxG: _toDouble(json['proteinMaxG']),
        proteinPerKgMin: _toDouble(json['proteinPerKgMin']),
        proteinPerKgMax: _toDouble(json['proteinPerKgMax']),
        fatFreeMassKg: _toDouble(json['fatFreeMassKg']),
        maintenanceKcal: _toDouble(json['maintenanceKcal']),
        restingKcal: _toDouble(json['restingKcal']),
        activityKcal: _toDouble(json['activityKcal']),
        estimated: json['estimated'] == true,
        needsHeight: json['needsHeight'] == true,
      );
}

/// A range of days of logged nutrition, with the averages over the days that
/// were logged and the member's targets. The averages come from the API, like
/// the step average, so the phone and the web app cannot disagree.
class NutritionSeries {
  final String provider;
  final String range;
  final String from;
  final String to;
  final List<NutritionDay> days;
  final double avgKcal;
  final double avgProteinG;
  final int loggedDays;
  final NutritionTargets? targets;

  const NutritionSeries({
    this.provider = '',
    this.range = '',
    this.from = '',
    this.to = '',
    this.days = const [],
    this.avgKcal = 0,
    this.avgProteinG = 0,
    this.loggedDays = 0,
    this.targets,
  });

  factory NutritionSeries.fromJson(Map<String, dynamic> json) => NutritionSeries(
        provider: _toString(json['provider']),
        range: _toString(json['range']),
        from: _toString(json['from']),
        to: _toString(json['to']),
        days: (json['days'] as List? ?? [])
            .map((item) => NutritionDay.fromJson(Map<String, dynamic>.from(item as Map)))
            .toList(),
        avgKcal: _toDouble(json['avgKcal']),
        avgProteinG: _toDouble(json['avgProteinG']),
        loggedDays: _toInt(json['loggedDays']),
        targets: json['targets'] == null
            ? null
            : NutritionTargets.fromJson(Map<String, dynamic>.from(json['targets'] as Map)),
      );
}

class OneRm {
  final String exerciseId;
  final String slug;
  final Map<String, String> labels;
  final double value;
  final DateTime updatedAt;

  OneRm({
    required this.exerciseId,
    this.slug = '',
    this.labels = const {},
    this.value = 0,
    DateTime? updatedAt,
  }) : updatedAt = updatedAt ?? DateTime.utc(1970);

  factory OneRm.fromJson(Map<String, dynamic> json) => OneRm(
        exerciseId: _toString(json['exerciseId']),
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        value: _toDouble(json['value']),
        updatedAt: _toDate(json['updatedAt']),
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// What the member logged for one prescribed set.
class SetLog {
  final int? actualReps;
  final double? actualRpe;
  final double? actualLoad;

  /// Whether the member ran this set without their belt. Only asked about on a
  /// movement that would normally be belted (see [ProgramSet.withBelt]).
  final bool beltless;
  final String comment;
  final bool done;
  final double e1rm;

  const SetLog({
    this.actualReps,
    this.actualRpe,
    this.actualLoad,
    this.beltless = false,
    this.comment = '',
    this.done = false,
    this.e1rm = 0,
  });

  factory SetLog.fromJson(Map<String, dynamic> json) => SetLog(
        actualReps: _toIntOrNull(json['actualReps']),
        actualRpe: _toDoubleOrNull(json['actualRpe']),
        actualLoad: _toDoubleOrNull(json['actualLoad']),
        beltless: json['beltless'] == true,
        comment: _toString(json['comment']),
        done: json['done'] == true,
        e1rm: _toDouble(json['e1rm']),
      );
}

/// One prescribed set, with the load already resolved against this member's own
/// 1RM. [loadKnown] is false when the max it would come from hasn't been
/// recorded yet - the UI shows the spreadsheet's "?" for those.
class ProgramSet {
  final String id;
  final String exerciseId;
  final String exerciseSlug;

  /// Whether this movement is one a lifter wears a belt for - the squat and
  /// the deadlift and their variations, decided by the API off the same table
  /// the loads come from. It is what puts a beltless switch on the set.
  final bool withBelt;
  final Map<String, String> exerciseLabels;
  final String exerciseOneRmRef;
  final bool bodyweight;
  final int reps;
  final double? rpe;
  final double? percentage;
  final double? absoluteLoad;
  final String loadMode;
  final double load;
  final double roundedLoad;
  final double computedPercentage;
  final bool loadKnown;
  final double oneRm;
  /// True when [load] came from a max the member demonstrated earlier in this
  /// same session rather than from the 1RM on file - so the screen can say
  /// where the number came from instead of the weight seeming to move by
  /// itself.
  final bool autoregulated;
  final String notes;
  final SetLog? log;

  const ProgramSet({
    required this.id,
    this.exerciseId = '',
    this.exerciseSlug = '',
    this.withBelt = false,
    this.exerciseLabels = const {},
    this.exerciseOneRmRef = '',
    this.bodyweight = false,
    this.reps = 0,
    this.rpe,
    this.percentage,
    this.absoluteLoad,
    this.loadMode = '',
    this.load = 0,
    this.roundedLoad = 0,
    this.computedPercentage = 0,
    this.loadKnown = false,
    this.oneRm = 0,
    this.autoregulated = false,
    this.notes = '',
    this.log,
  });

  factory ProgramSet.fromJson(Map<String, dynamic> json) => ProgramSet(
        id: _toString(json['id']),
        exerciseId: _toString(json['exerciseId']),
        exerciseSlug: _toString(json['exerciseSlug']),
        withBelt: json['withBelt'] == true,
        exerciseLabels: _toLabels(json['exerciseLabels']),
        exerciseOneRmRef: _toString(json['exerciseOneRmRef']),
        bodyweight: json['bodyweight'] == true,
        reps: _toInt(json['reps']),
        rpe: _toDoubleOrNull(json['rpe']),
        percentage: _toDoubleOrNull(json['percentage']),
        absoluteLoad: _toDoubleOrNull(json['absoluteLoad']),
        loadMode: _toString(json['loadMode']),
        load: _toDouble(json['load']),
        roundedLoad: _toDouble(json['roundedLoad']),
        computedPercentage: _toDouble(json['computedPercentage']),
        loadKnown: json['loadKnown'] == true,
        oneRm: _toDouble(json['oneRm']),
        autoregulated: json['autoregulated'] == true,
        notes: _toString(json['notes']),
        log: json['log'] is Map<String, dynamic> ? SetLog.fromJson(json['log']) : null,
      );

  String label(String locale) => exerciseLabels[locale]?.isNotEmpty == true
      ? exerciseLabels[locale]!
      : (exerciseLabels['en']?.isNotEmpty == true ? exerciseLabels['en']! : exerciseSlug);

  bool get isBodyweight => loadMode == 'bodyweight';
}

/// A program as its coach sees it in the club's list.
/// The audiences a program can have, widest last, as the API names them.
const programVisibilities = ['private', 'club', 'public'];

/// Maps whatever the payload carries onto one of them.
///
/// Club-only is the fallback for the same reason it is on the API: an
/// unrecognized value must never widen an audience. A blank string counts as
/// unrecognized, which a plain null-fallback would have let through.
String programVisibility(dynamic value) {
  final text = value?.toString() ?? '';
  return programVisibilities.contains(text) ? text : 'club';
}

class Program {
  final String id;
  final String clubId;
  final String name;
  final String description;
  /// Who may read it: 'private', 'club' or 'public'. Anything else - including
  /// nothing at all, on a program written before sharing existed - reads as
  /// club-only, matching the API's own normalization.
  final String visibility;
  final String authorName;
  final int weeks;
  final int dayCount;
  final int setCount;

  const Program({
    required this.id,
    this.clubId = '',
    this.name = '',
    this.description = '',
    this.visibility = 'club',
    this.authorName = '',
    this.weeks = 0,
    this.dayCount = 0,
    this.setCount = 0,
  });

  factory Program.fromJson(Map<String, dynamic> json) => Program(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        name: _toString(json['name']),
        description: _toString(json['description']),
        visibility: programVisibility(json['visibility']),
        authorName: _toString(json['authorName']),
        weeks: _toInt(json['weeks']),
        dayCount: _toInt(json['dayCount']),
        setCount: _toInt(json['setCount']),
      );
}

/// A program with its sessions, as returned to a coach authoring it. The loads
/// on each set are resolved against the *coach's* own maxes here, which is what
/// makes the numbers concrete while writing.
class ProgramDetail {
  final Program program;
  final List<ProgramDay> days;

  const ProgramDetail({required this.program, this.days = const []});

  factory ProgramDetail.fromJson(Map<String, dynamic> json) => ProgramDetail(
        program: Program.fromJson(json),
        days: (json['days'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramDay.fromJson)
            .toList(),
      );
}

class ProgramDay {
  final String id;
  final int week;
  final int day;
  final String title;
  final List<ProgramSet> sets;

  const ProgramDay({
    required this.id,
    this.week = 0,
    this.day = 0,
    this.title = '',
    this.sets = const [],
  });

  factory ProgramDay.fromJson(Map<String, dynamic> json) => ProgramDay(
        id: _toString(json['id']),
        week: _toInt(json['week']),
        day: _toInt(json['day']),
        title: _toString(json['title']),
        sets: (json['sets'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramSet.fromJson)
            .toList(),
      );

  int get doneCount => sets.where((set) => set.log?.done == true).length;

  /// The name a coach gave this session, or '' when it has none of its own.
  ///
  /// A session saved without a title gets one from the API - the English
  /// "Week 2 Day 3" - so that exports have something to print. Showing that
  /// back would be redundant beside the week and day already on the header,
  /// and in the wrong language for a reader in French or Arabic. Only a title
  /// somebody actually typed is theirs to see.
  String get name {
    final trimmed = title.trim();
    return trimmed == 'Week $week Day $day' ? '' : trimmed;
  }
}

/// A program assigned to the member, as listed on the training screen.
class Assignment {
  final String id;
  final String programId;
  final String programName;
  final String clubName;
  final String status;
  final String note;
  final String startDate;
  final int completedSets;
  final int totalSets;

  const Assignment({
    required this.id,
    this.programId = '',
    this.programName = '',
    this.clubName = '',
    this.status = 'active',
    this.note = '',
    this.startDate = '',
    this.completedSets = 0,
    this.totalSets = 0,
  });

  factory Assignment.fromJson(Map<String, dynamic> json) => Assignment(
        id: _toString(json['id']),
        programId: _toString(json['programId']),
        programName: _toString(json['programName']),
        clubName: _toString(json['clubName']),
        status: _toString(json['status'], 'active'),
        note: _toString(json['note']),
        startDate: _toString(json['startDate']),
        completedSets: _toInt(json['completedSets']),
        totalSets: _toInt(json['totalSets']),
      );
}

/// One assigned program with every session resolved for the member running it.
class AssignmentDetail {
  final Assignment assignment;
  final List<ProgramDay> days;
  final List<Exercise> missingOneRms;
  /// What the AI already said about these sessions, by day id, newest first -
  /// a session can be read as many times as somebody asks. Sent with the
  /// block rather than fetched per session, so a twelve-week program does not
  /// cost forty requests to find the three readings that exist.
  final Map<String, List<SessionFeedback>> feedback;

  const AssignmentDetail({
    required this.assignment,
    this.days = const [],
    this.missingOneRms = const [],
    this.feedback = const {},
  });

  factory AssignmentDetail.fromJson(Map<String, dynamic> json) => AssignmentDetail(
        assignment: Assignment.fromJson(json),
        days: (json['days'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramDay.fromJson)
            .toList(),
        missingOneRms: (json['missingOneRms'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(Exercise.fromJson)
            .toList(),
        feedback: {
          for (final entry in (json['feedback'] as Map? ?? const {}).entries)
            entry.key.toString(): (entry.value as List? ?? const [])
                .whereType<Map<String, dynamic>>()
                .map(SessionFeedback.fromJson)
                .toList(),
        },
      );
}

/// One address an account has connected from, with how often.
class ConnectionIp {
  final String ip;
  final int count;
  final DateTime firstSeen;
  final DateTime lastSeen;

  const ConnectionIp({
    this.ip = '',
    this.count = 0,
    required this.firstSeen,
    required this.lastSeen,
  });

  factory ConnectionIp.fromJson(Map<String, dynamic> json) => ConnectionIp(
        ip: _toString(json['ip']),
        count: _toInt(json['count']),
        firstSeen: _toDate(json['firstSeen']).toLocal(),
        lastSeen: _toDate(json['lastSeen']).toLocal(),
      );
}

/// An account that asked to be a coach at signup, waiting on a decision.
class CoachApplicant {
  final String id;
  final String email;
  final String name;
  final String surname;

  /// Their profile name, when they have picked one. The profile page opens by
  /// id just as well, so this is only what makes the address readable.
  final String handle;
  final String picture;
  final DateTime? requestedAt;

  /// Which role they applied for: 'coach' or 'nutritionist'. A request stored
  /// before the role was recorded reads as coach.
  final String role;

  const CoachApplicant({
    required this.id,
    this.email = '',
    this.name = '',
    this.surname = '',
    this.handle = '',
    this.picture = '',
    this.requestedAt,
    this.role = 'coach',
  });

  factory CoachApplicant.fromJson(Map<String, dynamic> json) {
    final request = json['request'] is Map<String, dynamic>
        ? json['request'] as Map<String, dynamic>
        : const <String, dynamic>{};
    return CoachApplicant(
      id: _toString(json['id']),
      email: _toString(json['email']),
      name: _toString(json['name']),
      surname: _toString(json['surname']),
      handle: _toString(json['handle']),
      picture: _toString(json['picture']),
      requestedAt: _optionalDate(request['requestedAt']),
      role: _toString(request['role'], 'coach') == 'nutritionist' ? 'nutritionist' : 'coach',
    );
  }

  String get fullName => '$name $surname'.trim();
}

/// A private thread with one other member.
///
/// There is exactly one per pair of people, so it is addressed by who is in it
/// rather than by an id the client has to look up first.
class Conversation {
  final String id;
  final User other;
  final String lastMessage;
  final String lastSenderId;
  final int unread;
  final DateTime updatedAt;

  const Conversation({
    required this.id,
    this.other = const User(id: ''),
    this.lastMessage = '',
    this.lastSenderId = '',
    this.unread = 0,
    required this.updatedAt,
  });

  factory Conversation.fromJson(Map<String, dynamic> json) => Conversation(
        id: _toString(json['id']),
        other: User.fromJson(json['other'] is Map<String, dynamic>
            ? json['other'] as Map<String, dynamic>
            : const {}),
        lastMessage: _toString(json['lastMessage']),
        lastSenderId: _toString(json['lastSenderId']),
        unread: _toInt(json['unread']),
        updatedAt: _toDate(json['updatedAt']).toLocal(),
      );
}

/// One line in a conversation.
class PrivateMessage {
  final String id;
  final String senderId;
  final User sender;
  final String content;

  /// Pictures carried inline as base64 data URIs, the way a post's are.
  final List<String> pictures;

  /// Derived from the content by the API: whatever URL was pasted is the
  /// message's embed, so a video shared here plays like one shared in the feed.
  final List<String> links;

  /// A voice message's URL, in the sender's own storage.
  final String audio;

  /// Whether this side wrote it - which is all the UI needs to decide where to
  /// draw the bubble.
  final bool mine;

  /// Whether this caller may remove it: its sender, or a superadmin. Reported
  /// by the API rather than derived here, so the two clients cannot each carry
  /// their own copy of the rule.
  final bool deletable;
  final DateTime createdAt;

  const PrivateMessage({
    required this.id,
    this.senderId = '',
    this.sender = const User(id: ''),
    this.content = '',
    this.deletable = false,
    this.pictures = const [],
    this.links = const [],
    this.audio = '',
    this.mine = false,
    required this.createdAt,
  });

  factory PrivateMessage.fromJson(Map<String, dynamic> json) => PrivateMessage(
        id: _toString(json['id']),
        senderId: _toString(json['senderId']),
        sender: User.fromJson(json['sender'] is Map<String, dynamic>
            ? json['sender'] as Map<String, dynamic>
            : const {}),
        content: _toString(json['content']),
        pictures: _toStringList(json['pictures']),
        links: _toStringList(json['links']),
        audio: _toString(json['audio']),
        mine: json['mine'] == true,
        deletable: json['deletable'] == true,
        createdAt: _toDate(json['createdAt']).toLocal(),
      );
}

/// One open thread: who it is with, and what has been said.
class Thread {
  final String conversationId;
  final User other;
  final List<PrivateMessage> messages;

  const Thread({
    this.conversationId = '',
    this.other = const User(id: ''),
    this.messages = const [],
  });

  factory Thread.fromJson(Map<String, dynamic> json) => Thread(
        conversationId: _toString(json['conversationId']),
        other: User.fromJson(json['other'] is Map<String, dynamic>
            ? json['other'] as Map<String, dynamic>
            : const {}),
        messages: (json['messages'] is List ? json['messages'] as List : const [])
            .map((item) => PrivateMessage.fromJson(
                item is Map<String, dynamic> ? item : const <String, dynamic>{}))
            .toList(),
      );
}

/// A club asking somebody to join it.
///
/// Invitations are addressed by email rather than by user id, so one sent
/// before the account existed is still waiting when it does.
class Invitation {
  final String id;
  final String clubId;
  final String clubName;
  final String inviterName;
  final String role;
  final String status;
  final String message;
  final int memberCount;

  const Invitation({
    required this.id,
    this.clubId = '',
    this.clubName = '',
    this.inviterName = '',
    this.role = 'member',
    this.status = 'pending',
    this.message = '',
    this.memberCount = 0,
  });

  factory Invitation.fromJson(Map<String, dynamic> json) => Invitation(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        clubName: _toString(json['clubName']),
        inviterName: _toString(json['inviterName']),
        role: _toString(json['role'], 'member'),
        status: _toString(json['status'], 'pending'),
        message: _toString(json['message']),
        memberCount: _toInt(json['memberCount']),
      );
}

/// One entry in the calendar: a meet, a club session, a camp.
///
/// Times arrive as RFC 3339 instants and are kept in the device's local zone,
/// so a meet reads at the hour it actually starts wherever the athlete is.
class Event {
  final String id;
  final String clubId;
  final String clubName;
  final String title;
  final String description;
  final String location;
  final String url;
  final String kind;

  /// Hex the event is drawn in, as `#rrggbb`, or empty when it has none and
  /// the kind's own colour stands.
  final String color;
  /// Whether this entry occupies whole days rather than a time of day.
  final bool allDay;
  final DateTime startsAt;
  final DateTime? endsAt;
  final String visibility;
  final bool editable;
  final bool deletable;

  const Event({
    required this.id,
    this.clubId = '',
    this.clubName = '',
    this.title = '',
    this.description = '',
    this.location = '',
    this.url = '',
    this.kind = 'other',
    this.color = '',
    this.allDay = false,
    required this.startsAt,
    this.endsAt,
    this.visibility = 'public',
    this.editable = false,
    this.deletable = false,
  });

  factory Event.fromJson(Map<String, dynamic> json) => Event(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        clubName: _toString(json['clubName']),
        title: _toString(json['title']),
        description: _toString(json['description']),
        location: _toString(json['location']),
        url: _toString(json['url']),
        kind: _toString(json['kind'], 'other'),
        color: _toString(json['color']),
        // Kept in the device's own zone: the list is read as wall-clock time,
        // and converting on every render would be the same conversion done
        // repeatedly.
        allDay: json['allDay'] == true,
        startsAt: _toDate(json['startsAt']).toLocal(),
        endsAt: _optionalDate(json['endsAt']),
        visibility: _toString(json['visibility'], 'public'),
        editable: json['editable'] == true,
        deletable: json['deletable'] == true,
      );
}

class Post {
  final String id;
  final String authorId;
  final User author;

  /// The clubs a club-only post was shared with, empty for a public one.
  /// clubNames matches index for index, so a card can name them without a
  /// second request.
  final List<String> clubIds;
  final List<String> clubNames;
  final String content;
  final List<String> pictures;
  final List<String> links;
  final String visibility;
  final int likes;
  final bool liked;
  final int comments;
  final bool editable;
  final bool deletable;
  final DateTime createdAt;

  Post({
    required this.id,
    this.authorId = '',
    this.author = const User(id: ''),
    this.clubIds = const [],
    this.clubNames = const [],
    this.content = '',
    this.pictures = const [],
    this.links = const [],
    this.visibility = 'public',
    this.likes = 0,
    this.liked = false,
    this.comments = 0,
    this.editable = false,
    this.deletable = false,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.utc(1970);

  factory Post.fromJson(Map<String, dynamic> json) => Post(
        id: _toString(json['id']),
        authorId: _toString(json['authorId']),
        author: json['author'] is Map<String, dynamic>
            ? User.fromJson(json['author'])
            : const User(id: ''),
        clubIds: _toStringList(json['clubIds']),
        clubNames: _toStringList(json['clubNames']),
        content: _toString(json['content']),
        pictures: _toStringList(json['pictures']),
        links: _toStringList(json['links']),
        visibility: _toString(json['visibility'], 'public'),
        likes: _toInt(json['likes']),
        liked: json['liked'] == true,
        comments: _toInt(json['comments']),
        editable: json['editable'] == true,
        deletable: json['deletable'] == true,
        createdAt: _toDate(json['createdAt']),
      );

  Post copyWith({int? likes, bool? liked, int? comments}) => Post(
        id: id,
        authorId: authorId,
        author: author,
        clubIds: clubIds,
        clubNames: clubNames,
        content: content,
        pictures: pictures,
        links: links,
        visibility: visibility,
        likes: likes ?? this.likes,
        liked: liked ?? this.liked,
        comments: comments ?? this.comments,
        editable: editable,
        deletable: deletable,
        createdAt: createdAt,
      );
}

class Comment {
  final String id;
  final User author;
  final String content;
  final bool editable;
  final bool deletable;
  final DateTime createdAt;
  final DateTime updatedAt;

  Comment({
    required this.id,
    this.author = const User(id: ''),
    this.content = '',
    this.editable = false,
    this.deletable = false,
    DateTime? createdAt,
    DateTime? updatedAt,
  })  : createdAt = createdAt ?? DateTime.utc(1970),
        updatedAt = updatedAt ?? DateTime.utc(1970);

  /// Whether it has been changed since it was written.
  ///
  /// Strict, and it can be: both timestamps default to now(), which Postgres
  /// holds fixed for the whole transaction, so a freshly written row carries
  /// two identical stamps.
  bool get wasEdited => updatedAt.isAfter(createdAt);

  factory Comment.fromJson(Map<String, dynamic> json) => Comment(
        id: _toString(json['id']),
        author: json['author'] is Map<String, dynamic>
            ? User.fromJson(json['author'])
            : const User(id: ''),
        content: _toString(json['content']),
        editable: json['editable'] == true,
        deletable: json['deletable'] == true,
        createdAt: _toDate(json['createdAt']),
        updatedAt: _toDate(json['updatedAt']),
      );
}

/// One row of a member search.
///
/// A projection, not an account: the API decides what a caller may see of
/// somebody else and sends only that, so a result carries no more than a name,
/// a picture and the two badges - and [sharesClub], which is usually the
/// answer to "why is this person in my results".
class MemberResult {
  final String id;

  /// Empty when the member has no profile name. The profile page is reached by
  /// handle, so a result without one can only be messaged.
  final String handle;
  final String name;
  final String surname;
  final String role;
  final String picture;

  /// Whether this person and the caller are in the same club.
  final bool sharesClub;

  /// Whether the name above is a username rather than a person's name.
  final bool anonymous;

  const MemberResult({
    required this.id,
    this.handle = '',
    this.name = '',
    this.surname = '',
    this.role = '',
    this.picture = '',
    this.sharesClub = false,
    this.anonymous = false,
  });

  String get fullName => '$name $surname'.trim();

  factory MemberResult.fromJson(Map<String, dynamic> json) => MemberResult(
        id: _toString(json['id']),
        handle: _toString(json['handle']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        role: _toString(json['role']),
        picture: _toString(json['picture']),
        sharesClub: json['sharesClub'] == true,
        anonymous: json['anonymous'] == true,
      );
}

/// What a lifter's total is worth: the three coefficients, the tier it puts
/// them in, where it sits among the members here, and what it has earned.
///
/// Every number is the API's (internal/strength). Nothing here recomputes a
/// coefficient - three fitted polynomials in two languages would be two
/// answers to the same question.
class StrengthResult {
  final double total;
  final double dots;
  final double wilks;
  final double ipfGl;
  final String tierKey;
  final int percentile;

  /// How many members the percentile is out of. A percentile among four people
  /// is a fact about four people, so the count travels with it.
  final int sample;
  final List<StrengthBadge> badges;

  const StrengthResult({
    this.total = 0,
    this.dots = 0,
    this.wilks = 0,
    this.ipfGl = 0,
    this.tierKey = '',
    this.percentile = 0,
    this.sample = 0,
    this.badges = const [],
  });

  factory StrengthResult.fromJson(Map<String, dynamic> json) {
    final scores = json['scores'] is Map<String, dynamic>
        ? json['scores'] as Map<String, dynamic>
        : const <String, dynamic>{};
    final tier = json['tier'] is Map<String, dynamic>
        ? json['tier'] as Map<String, dynamic>
        : const <String, dynamic>{};
    final percentile = json['percentile'] is Map<String, dynamic>
        ? json['percentile'] as Map<String, dynamic>
        : const <String, dynamic>{};
    return StrengthResult(
      total: _toDouble(json['total']),
      dots: _toDouble(scores['dots']),
      wilks: _toDouble(scores['wilks']),
      ipfGl: _toDouble(scores['ipfGl']),
      tierKey: _toString(tier['key']),
      percentile: _toInt(percentile['value']),
      sample: _toInt(percentile['sample']),
      badges: (json['badges'] as List? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(StrengthBadge.fromJson)
          .toList(),
    );
  }

  List<StrengthBadge> get earned => badges.where((badge) => badge.earned).toList();
}

/// One achievement, earned or not. An unearned one carries how far along it is,
/// because a target you can see is worth more than one that appears out of
/// nowhere the day you hit it.
class StrengthBadge {
  final String key;
  final String kind;
  final bool earned;
  final double progress;

  const StrengthBadge({
    required this.key,
    this.kind = '',
    this.earned = false,
    this.progress = 0,
  });

  factory StrengthBadge.fromJson(Map<String, dynamic> json) => StrengthBadge(
        key: _toString(json['key']),
        kind: _toString(json['kind']),
        earned: json['earned'] == true,
        progress: _toDouble(json['progress']),
      );
}

/// A member's or coach's public profile.
class PublicProfile {
  final String id;
  final String handle;
  final String name;
  final String surname;
  final String role;
  /// Where else to find them, keyed as the API names them.
  final Map<String, String> socials;
  final String bio;
  final String picture;
  final double bodyweight;
  final List<ProfileBest> bests;
  final double total;
  final int followers;
  final int following;
  final bool followed;

  /// Whether to offer this profile's fitness data (charts, export, AI reading):
  /// the member, a superadmin, a coach or nutritionist of a shared club, or
  /// anyone when the member made it public. [canAdviseHealth] is the narrower
  /// right to generate the AI reading (not a public visitor), and
  /// [fitnessPublic] says the data is open to everyone.
  final bool canViewHealth;
  final bool canAdviseHealth;
  final bool fitnessPublic;

  /// What their own maxes are worth. Null for somebody who has not weighed in
  /// or entered no lifts: a profile with nothing to measure wears nothing.
  final StrengthResult? strength;

  const PublicProfile({
    required this.id,
    this.handle = '',
    this.name = '',
    this.surname = '',
    this.role = '',
    this.socials = const {},
    this.bio = '',
    this.picture = '',
    this.bodyweight = 0,
    this.bests = const [],
    this.total = 0,
    this.followers = 0,
    this.following = 0,
    this.followed = false,
    this.canViewHealth = false,
    this.canAdviseHealth = false,
    this.fitnessPublic = false,
    this.strength,
  });

  factory PublicProfile.fromJson(Map<String, dynamic> json) => PublicProfile(
        id: _toString(json['id']),
        handle: _toString(json['handle']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        role: _toString(json['role']),
        strength: json['strength'] is Map<String, dynamic>
            ? StrengthResult.fromJson(json['strength'] as Map<String, dynamic>)
            : null,
        socials: _toLabels(json['socials']),
        bio: _toString(json['bio']),
        picture: _toString(json['picture']),
        bodyweight: _toDouble(json['bodyweight']),
        bests: (json['bests'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProfileBest.fromJson)
            .toList(),
        total: _toDouble(json['total']),
        followers: _toInt(json['followers']),
        following: _toInt(json['following']),
        followed: json['followed'] == true,
        canViewHealth: json['canViewHealth'] == true,
        canAdviseHealth: json['canAdviseHealth'] == true,
        fitnessPublic: json['fitnessPublic'] == true,
      );

  String get fullName => '$name $surname'.trim();
}

class ProfileBest {
  final String slug;
  final Map<String, String> labels;
  final double value;

  const ProfileBest({this.slug = '', this.labels = const {}, this.value = 0});

  factory ProfileBest.fromJson(Map<String, dynamic> json) => ProfileBest(
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        value: _toDouble(json['value']),
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// A paginated listing.
class Page<T> {
  final List<T> results;
  final int totalResults;

  const Page({this.results = const [], this.totalResults = 0});

  factory Page.fromJson(Map<String, dynamic> json, T Function(Map<String, dynamic>) parse) => Page(
        results: (json['results'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(parse)
            .toList(),
        totalResults: _toInt(json['totalResults']),
      );
}

// --- the AI assistant ---

/// One account's connection to a language model.
///
/// Shaped on a storage down to the sharing, because it is the same
/// arrangement: an account holds a credential and decides who else may spend
/// it. The key itself never leaves the server - [hasToken] is what a form
/// needs to know, so it can offer to replace one rather than silently wiping
/// it.
class AiConnection {
  final String id;
  final String ownerId;
  final String name;
  final String provider;
  final String baseUrl;
  final String model;

  /// How long an answer from this subscription may run, in tokens. Zero means
  /// the server default - a form shows it as an empty field with the default as
  /// the placeholder, rather than a number the member never chose.
  final int maxTokens;
  final bool hasToken;
  final String role;
  final String ownerName;

  const AiConnection({
    this.id = '',
    this.ownerId = '',
    this.name = '',
    this.provider = '',
    this.baseUrl = '',
    this.model = '',
    this.maxTokens = 0,
    this.hasToken = false,
    this.role = '',
    this.ownerName = '',
  });

  factory AiConnection.fromJson(Map<String, dynamic> json) => AiConnection(
        id: _toString(json['id']),
        ownerId: _toString(json['ownerId']),
        name: _toString(json['name']),
        provider: _toString(json['provider']),
        baseUrl: _toString(json['baseUrl']),
        model: _toString(json['model']),
        maxTokens: _toInt(json['maxTokens']),
        hasToken: json['hasToken'] == true,
        role: _toString(json['role']),
        ownerName: _toString(json['ownerName']),
      );
}

/// One custom header on an HTTP or WebSocket callback: a name and a value.
class CallbackHeader {
  final String name;
  final String value;
  const CallbackHeader({required this.name, required this.value});

  factory CallbackHeader.fromJson(Map<String, dynamic> json) =>
      CallbackHeader(name: _toString(json['name']), value: _toString(json['value']));

  Map<String, dynamic> toJson() => {'name': name, 'value': value};
}

/// A member's recording callback: a trigger the phone fires when a bar-speed
/// set starts, so a device of their own - an ESP32 on the gym network - records
/// it too. A webhook (HTTP POST) is one transport; a WebSocket message and an
/// MQTT publish are the others. The secrets ([token], [password]) come back from
/// the server because the phone needs them to fire, and a callback is the
/// member's own device, shared with no one.
class RecordingCallback {
  final String id;
  final String ownerId;
  final String name;
  final String type; // http | websocket | mqtt
  final String url;

  final String token;
  final bool hasToken;
  final List<CallbackHeader> headers;

  final String username;
  final String password;
  final bool hasPassword;

  final String topic;
  final String clientId;
  final int qos;

  final bool enabled;

  const RecordingCallback({
    this.id = '',
    this.ownerId = '',
    this.name = '',
    this.type = 'http',
    this.url = '',
    this.token = '',
    this.hasToken = false,
    this.headers = const [],
    this.username = '',
    this.password = '',
    this.hasPassword = false,
    this.topic = '',
    this.clientId = '',
    this.qos = 0,
    this.enabled = true,
  });

  /// What to show for it: its name, or the URL when it has none.
  String get label => name.trim().isNotEmpty ? name.trim() : url;

  factory RecordingCallback.fromJson(Map<String, dynamic> json) => RecordingCallback(
        id: _toString(json['id']),
        ownerId: _toString(json['ownerId']),
        name: _toString(json['name']),
        type: _toString(json['type']),
        url: _toString(json['url']),
        token: _toString(json['token']),
        hasToken: json['hasToken'] == true,
        headers: (json['headers'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(CallbackHeader.fromJson)
            .toList(),
        username: _toString(json['username']),
        password: _toString(json['password']),
        hasPassword: json['hasPassword'] == true,
        topic: _toString(json['topic']),
        clientId: _toString(json['clientId']),
        qos: _toInt(json['qos']),
        enabled: json['enabled'] == true,
      );
}

/// One row of the server's provider catalogue: where a provider lives, what
/// it serves, and what its key looks like.
///
/// The form fills itself in from this as the member picks, and its
/// placeholders are that provider's own values rather than a fixed example of
/// somebody else's. The label is vendor branding, the same in every language,
/// which is why it comes from the server and not from the dictionaries.
class AiProvider {
  final String name;
  final String label;
  final String defaultUrl;
  final String defaultModel;

  /// Models to suggest, best first. A suggestion, never a constraint: the
  /// API takes whatever is typed.
  final List<String> models;

  /// What this provider's key looks like, for the field's placeholder.
  final String keyHint;

  const AiProvider({
    this.name = '',
    this.label = '',
    this.defaultUrl = '',
    this.defaultModel = '',
    this.models = const [],
    this.keyHint = '',
  });

  factory AiProvider.fromJson(Map<String, dynamic> json) => AiProvider(
        name: _toString(json['name']),
        label: _toString(json['label'], _toString(json['name'])),
        defaultUrl: _toString(json['defaultUrl']),
        defaultModel: _toString(json['defaultModel']),
        models: (json['models'] as List? ?? const []).map(_toString).toList(),
        keyHint: _toString(json['keyHint']),
      );
}

/// One line of an access list, as the owner reads it.
///
/// The same shape for a storage and for an AI connection, because it is the
/// same arrangement: somebody holds a credential and decides who else may
/// spend it. Only the roles differ, and those are the caller's to read.
class AccessGrant {
  final String userId;
  final String name;
  final String handle;
  final String role;

  const AccessGrant({this.userId = '', this.name = '', this.handle = '', this.role = ''});

  factory AccessGrant.fromJson(Map<String, dynamic> json) => AccessGrant(
        userId: _toString(json['userId']),
        name: _toString(json['name']),
        handle: _toString(json['handle']),
        role: _toString(json['role']),
      );
}

/// What a credential reads as once it is stored: the API never sends a secret
/// back, and echoing this marker on save means "leave the stored one alone".
const secretSet = '__set__';

/// Where a member's videos and voice messages are written.
///
/// An upload goes to every one of their targets; the link in the post comes
/// from the first, which is why the order is a setting rather than a display
/// preference.
class Storage {
  final String id;
  final String ownerId;
  final StorageConnection connection;
  final int position;

  /// The caller's own role on it: owner, writer or reader.
  final String role;
  final String ownerName;

  const Storage({
    this.id = '',
    this.ownerId = '',
    this.connection = const StorageConnection(),
    this.position = 0,
    this.role = '',
    this.ownerName = '',
  });

  factory Storage.fromJson(Map<String, dynamic> json) => Storage(
        id: _toString(json['id']),
        ownerId: _toString(json['ownerId']),
        connection: StorageConnection.fromJson(
            json['connection'] is Map<String, dynamic> ? json['connection'] : const {}),
        position: _toInt(json['position']),
        role: _toString(json['role']),
        ownerName: _toString(json['ownerName']),
      );
}

/// One target's settings: a bucket, or a Drive folder.
class StorageConnection {
  final String type;
  final String endpoint;
  final String bucketName;
  final String region;
  final String accessKey;

  /// Secrets come back as [secretSet] and are never readable. Sending the
  /// marker back keeps what is stored.
  final String secretKey;
  final String serviceAccountBase64;
  final String folderId;
  final String path;
  final String publicBaseUrl;

  /// Whether the bucket refuses public objects - in which case StrongFish
  /// serves its files itself, to readers it has checked.
  final bool private;

  const StorageConnection({
    this.type = 's3',
    this.endpoint = '',
    this.bucketName = '',
    this.region = '',
    this.accessKey = '',
    this.secretKey = '',
    this.serviceAccountBase64 = '',
    this.folderId = '',
    this.path = '',
    this.publicBaseUrl = '',
    this.private = false,
  });

  factory StorageConnection.fromJson(Map<String, dynamic> json) => StorageConnection(
        type: _toString(json['type'], 's3'),
        endpoint: _toString(json['endpoint']),
        bucketName: _toString(json['bucketName']),
        region: _toString(json['region']),
        accessKey: _toString(json['accessKey']),
        secretKey: _toString(json['secretKey']),
        serviceAccountBase64: _toString(json['serviceAccountBase64']),
        folderId: _toString(json['folderId']),
        path: _toString(json['path']),
        publicBaseUrl: _toString(json['publicBaseUrl']),
        private: json['private'] == true,
      );

  bool get isDrive => type == 'google_drive';

  /// Whether a credential is already stored, so a form can offer to replace
  /// it rather than showing a field that would wipe it.
  bool get hasSecret => secretKey == secretSet || serviceAccountBase64 == secretSet;

  Map<String, dynamic> toJson() => {
        'type': type,
        'endpoint': endpoint,
        'bucketName': bucketName,
        'region': region,
        'accessKey': accessKey,
        'secretKey': secretKey,
        'serviceAccountBase64': serviceAccountBase64,
        'folderId': folderId,
        'path': path,
        'publicBaseUrl': publicBaseUrl,
        'private': private,
      };

  /// One target in a line: enough to tell two buckets apart.
  String get summary => [
        if (isDrive) folderId else bucketName,
        if (isDrive) '' else endpoint,
        path,
      ].where((part) => part.isNotEmpty).join(' · ');
}

/// One question and the answer it got.
class AiExchange {
  final String id;
  final String question;
  final String answer;
  final String provider;
  final String model;

  /// The subscription it was spent on, as it was named at the time: with a
  /// picker above the thread, two answers in one conversation can come from
  /// two different keys and two different models.
  final String connection;
  final DateTime createdAt;

  const AiExchange({
    this.id = '',
    this.question = '',
    this.answer = '',
    this.provider = '',
    this.model = '',
    this.connection = '',
    required this.createdAt,
  });

  factory AiExchange.fromJson(Map<String, dynamic> json) => AiExchange(
        id: _toString(json['id']),
        question: _toString(json['question']),
        answer: _toString(json['answer']),
        provider: _toString(json['provider']),
        model: _toString(json['model']),
        connection: _toString(json['connection']),
        createdAt: _toDate(json['createdAt']),
      );

  /// What the credit calls whatever produced it: the model, or the
  /// subscription when the provider reported no model at all.
  String get credited => model.isNotEmpty ? model : connection;
}

/// A thread of exchanges: what the list shows, and what a thread holds.
class AiConversation {
  final String id;
  final String title;
  final bool shared;
  final String shareUrl;
  final int count;
  final List<AiExchange> exchanges;
  final DateTime updatedAt;

  const AiConversation({
    this.id = '',
    this.title = '',
    this.shared = false,
    this.shareUrl = '',
    this.count = 0,
    this.exchanges = const [],
    required this.updatedAt,
  });

  factory AiConversation.fromJson(Map<String, dynamic> json) => AiConversation(
        id: _toString(json['id']),
        title: _toString(json['title']),
        shared: json['shared'] == true,
        shareUrl: _toString(json['shareUrl']),
        count: _toInt(json['count']),
        exchanges: (json['exchanges'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(AiExchange.fromJson)
            .toList(),
        updatedAt: _toDate(json['updatedAt']),
      );
}

/// What the model said about one training session, as stored.
class SessionFeedback {
  final String id;
  final String dayId;
  final String authorName;
  final String content;
  final String provider;
  final String model;

  /// The subscription that paid for it, as it was named at the time - what
  /// the credit line under the reading says.
  final String connection;
  final DateTime createdAt;
  final DateTime updatedAt;

  const SessionFeedback({
    this.id = '',
    this.dayId = '',
    this.authorName = '',
    this.content = '',
    this.provider = '',
    this.model = '',
    this.connection = '',
    required this.createdAt,
    required this.updatedAt,
  });

  factory SessionFeedback.fromJson(Map<String, dynamic> json) => SessionFeedback(
        id: _toString(json['id']),
        dayId: _toString(json['dayId']),
        authorName: _toString(json['authorName']),
        content: _toString(json['content']),
        provider: _toString(json['provider']),
        model: _toString(json['model']),
        connection: _toString(json['connection']),
        createdAt: _toDate(json['createdAt']),
        updatedAt: _toDate(json['updatedAt']),
      );

  /// What the credit calls whatever produced it: the subscription, or - for a
  /// reading made before those were recorded - the model itself.
  String get credited => connection.isNotEmpty ? connection : model;
}

/// What the model said about the member's health data.
///
/// [nextAt] is when another reading may be asked for: the API rate-limits it
/// to one a day, and a button that says when it comes back is better than one
/// that only refuses when pressed.
class HealthFeedback {
  final String id;
  final String source;
  final String content;
  final String provider;
  final String model;

  /// The subscription that paid for it (see SessionFeedback.connection).
  final String connection;
  final int days;
  final DateTime updatedAt;

  const HealthFeedback({
    this.id = '',
    this.source = '',
    this.content = '',
    this.provider = '',
    this.model = '',
    this.connection = '',
    this.days = 0,
    required this.updatedAt,
  });

  factory HealthFeedback.fromJson(Map<String, dynamic> json) => HealthFeedback(
        id: _toString(json['id']),
        source: _toString(json['source']),
        content: _toString(json['content']),
        provider: _toString(json['provider']),
        model: _toString(json['model']),
        connection: _toString(json['connection']),
        days: _toInt(json['days']),
        updatedAt: _toDate(json['updatedAt']),
      );

  /// What the credit calls whatever produced it.
  String get credited => connection.isNotEmpty ? connection : model;
}
