import os, math, datetime
import sqlite3
import numpy as np
import pandas as pd

import tensorflow as tf
from tensorflow import keras
from tensorflow.keras import layers, regularizers
from tensorflow.keras.callbacks import EarlyStopping

# For hyperparameter tuning
import keras_tuner as kt

from sklearn.model_selection import KFold, train_test_split
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import accuracy_score, classification_report
from sklearn.utils import shuffle

from joblib import dump

#####################
#   CONFIG
#####################
DB_PATHS = [
    '/home/konomut/.trading/trades.db',
]
NUM_FOLDS = 3  # For K-fold CV

#####################
#  1) LOAD & MERGE
#####################

def load_merged_data():
    """
    Loads data from multiple SQLite databases, returning a single DataFrame.

    We print debug logs about how many rows are loaded from each DB.
    """
    all_data = []
    for db_path in DB_PATHS:
        if not os.path.exists(db_path):
            print(f"[DEBUG] DB path does not exist: {db_path}")
            continue

        try:
            conn = sqlite3.connect(db_path)
            query = """
            SELECT
              order_id,
              symbol,
              rsi as rsi_buy,
              macd as macd_buy,
              cci as cci_buy,
              mfi as mfi_buy,
              quantity as volume_buy,
              middleBound as middleBand_buy,
              upperBound as upperBand_buy,
              lowerBound as lowerBand_buy,
              profit_loss,
              timestamp as sell_timestamp
            FROM completed_trades
            WHERE order_id IS NOT NULL
            """
            df = pd.read_sql_query(query, conn)
            conn.close()

            print(f"[DEBUG] Loaded {len(df)} rows from {db_path}")
            if not df.empty:
                all_data.append(df)
        except Exception as e:
            print(f"[ERROR] loading data from {db_path}: {e}")

    if len(all_data) == 0:
        print("[DEBUG] No data loaded from any DBs.")
        return pd.DataFrame()

    merged = pd.concat(all_data, ignore_index=True)
    print(f"[DEBUG] Merged data shape after loading: {merged.shape}")
    return merged

#####################
#  2) FEATURE ENGINEERING
#####################

def feature_engineering(df):
    """
    We add some 'wild' example features:
      - Bollinger distance ratio: (middleBand_buy - lowerBand_buy)/(upperBand_buy - lowerBand_buy)
      - volume_log: log(volume_buy+1e-9)
    We create final label => profit_label=1 if profit_loss>0 else 0

    Removed references to buy_timestamp & time-of-day, as requested.
    """
    df = df.dropna().copy()

    # 1) Bollinger distance ratio
    df['bb_ratio'] = (df['middleBand_buy'] - df['lowerBand_buy']) / (
        (df['upperBand_buy'] - df['lowerBand_buy']) + 1e-9
    )

    # 2) volume_log
    df['volume_log'] = np.log(df['volume_buy'] + 1e-9)

    # 3) Final label => profit_label
    df['profit_label'] = (df['profit_loss'] > 0).astype(int)

    return df

#####################
#  3) BUILD & TUNE KERAS MODEL
#####################

def build_model(hp):
    """
    Keras model with hyperparameters from keras-tuner:
      - units_1 (32..128)
      - units_2 (16..64)
      - dropout_1, dropout_2 (0.2..0.5)
      - l2_1, l2_2 for L2 reg
      - lr for learning rate
    """
    model = keras.Sequential()

    # First layer
    units_1 = hp.Int('units_1', min_value=32, max_value=128, step=32)
    model.add(layers.Dense(
        units_1,
        activation='relu',
        kernel_regularizer=regularizers.l2(
            hp.Float('l2_1', 1e-6, 1e-3, sampling='log')
        )
    ))
    model.add(layers.Dropout(hp.Float('dropout_1', 0.2, 0.5, step=0.1)))

    # Second layer
    units_2 = hp.Int('units_2', min_value=16, max_value=64, step=16)
    model.add(layers.Dense(
        units_2,
        activation='relu',
        kernel_regularizer=regularizers.l2(
            hp.Float('l2_2', 1e-6, 1e-3, sampling='log')
        )
    ))
    model.add(layers.Dropout(hp.Float('dropout_2', 0.2, 0.5, step=0.1)))

    # Output layer => 1 (sigmoid)
    model.add(layers.Dense(1, activation='sigmoid'))

    # Learning rate
    lr = hp.Float('lr', 1e-4, 1e-2, sampling='log')
    optimizer = keras.optimizers.Adam(learning_rate=lr)
    model.compile(
        optimizer=optimizer,
        loss='binary_crossentropy',
        metrics=['accuracy']
    )
    return model

def run_kfold_cv(X, y, n_splits=5):
    """
    K-Fold cross-validation for performance estimates.
    For each fold:
      - Tune hyperparams w/ Keras Tuner
      - Train best model
      - Evaluate on test set
    """
    kf = KFold(n_splits=n_splits, shuffle=True, random_state=42)
    accuracies = []

    import keras_tuner as kt

    for fold, (train_idx, test_idx) in enumerate(kf.split(X, y)):
        print(f"\n[INFO] KFold fold={fold+1}/{n_splits}")
        X_train, X_test = X.iloc[train_idx], X.iloc[test_idx]
        y_train, y_test = y.iloc[train_idx], y.iloc[test_idx]

        # Scale
        scaler = StandardScaler()
        X_train_s = scaler.fit_transform(X_train)
        X_test_s  = scaler.transform(X_test)

        # Tuner for this fold
        tuner = kt.RandomSearch(
            build_model,
            objective='val_accuracy',
            max_trials=5,  # increase if you want deeper search
            executions_per_trial=1,
            overwrite=True,
            directory='ktuner_dir',
            project_name=f'fold_{fold}'
        )

        stop_early = EarlyStopping(monitor='val_loss', patience=3, restore_best_weights=True)

        tuner.search(
            X_train_s, y_train,
            epochs=20,
            validation_split=0.2,
            callbacks=[stop_early],
            verbose=0
        )

        best_hp = tuner.get_best_hyperparameters(num_trials=1)[0]
        print("[DEBUG] Best HP for this fold:", best_hp.values)

        # Build & train best model
        model = tuner.hypermodel.build(best_hp)
        model.fit(
            X_train_s, y_train,
            epochs=20,
            validation_split=0.2,
            callbacks=[stop_early],
            verbose=0
        )

        # Evaluate
        loss, acc = model.evaluate(X_test_s, y_test, verbose=0)
        print(f"[INFO] Fold={fold+1}, test accuracy={acc*100:.2f}%")
        accuracies.append(acc)

    print("\n=== CROSS-VAL RESULTS ===")
    print("Accuracies:", [f"{a*100:.2f}%" for a in accuracies])
    print(f"Mean acc={np.mean(accuracies)*100:.2f}%, std={np.std(accuracies)*100:.2f}%")

def do_hyperparam_search(X_train_s, y_train, project_name="final_model"):
    """
    Conduct a hyperparam search on the ENTIRE dataset for final training.
    """
    import keras_tuner as kt

    tuner = kt.RandomSearch(
        build_model,
        objective='val_accuracy',
        max_trials=5,
        executions_per_trial=1,
        overwrite=True,
        directory='ktuner_dir',
        project_name=project_name
    )

    stop_early = EarlyStopping(monitor='val_loss', patience=3, restore_best_weights=True)
    tuner.search(
        X_train_s, y_train,
        epochs=20,
        validation_split=0.2,
        callbacks=[stop_early],
        verbose=0
    )
    best_hp = tuner.get_best_hyperparameters(num_trials=1)[0]
    return best_hp

def train_final_model(X, y):
    """
    Train final model on the entire dataset (with an internal 80/20 split for val),
    then save model + scaler.
    """
    from sklearn.utils import shuffle
    X, y = shuffle(X, y, random_state=42)

    X_train, X_val, y_train, y_val = train_test_split(X, y, test_size=0.2, random_state=42, stratify=y)

    # Scale
    scaler = StandardScaler()
    X_train_s = scaler.fit_transform(X_train)
    X_val_s   = scaler.transform(X_val)

    # Hyperparam search
    best_hp = do_hyperparam_search(X_train_s, y_train, project_name="final_all_data")
    print("[DEBUG] best_hp final =>", best_hp.values)

    # Build final model
    model = build_model(best_hp)
    stop_early = EarlyStopping(monitor='val_loss', patience=3, restore_best_weights=True)

    model.fit(
        X_train_s, y_train,
        validation_data=(X_val_s, y_val),
        epochs=20,
        callbacks=[stop_early],
        verbose=1
    )

    # Evaluate on val
    val_loss, val_acc = model.evaluate(X_val_s, y_val, verbose=0)
    print(f"[INFO] Final Model Val Accuracy => {val_acc*100:.2f}%")

    # Save
    model.export("buy_outcome_keras_model.keras")
    print("[INFO] Keras model saved => buy_outcome_keras_model/")
    dump(scaler, "buy_outcome_keras_model_scaler.joblib")
    print("[INFO] Scaler saved => buy_outcome_keras_model_scaler.joblib")

def main():
    df = load_merged_data()
    if df.empty:
        print("[ERROR] No data => exit.")
        return

    df = feature_engineering(df)
    if df.empty:
        print("[ERROR] After feature eng => no data => exit.")
        return

    # Feature columns
    feature_cols = [
        'rsi_buy',
        'macd_buy',
        'cci_buy',
        'mfi_buy',
        'bb_ratio',
        'volume_log',
        # removed buy_hour_sin, buy_hour_cos
    ]
    X = df[feature_cols].copy()
    y = df['profit_label'].copy()

    # 1) K-Fold + Tuner => see how well it generalizes
    run_kfold_cv(X, y, n_splits=NUM_FOLDS)

    # 2) Final train & save
    train_final_model(X, y)

if __name__ == "__main__":
    main()
